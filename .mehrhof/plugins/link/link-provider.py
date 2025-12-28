#!/usr/bin/env python3
"""Link provider plugin for mehr - loads task content from any URL."""

import hashlib
import json
import re
import sys
import urllib.error
import urllib.request
from html.parser import HTMLParser
from urllib.parse import urlparse


class HTMLTextExtractor(HTMLParser):
    """Extract text content from HTML, stripping scripts and styles."""

    def __init__(self):
        super().__init__()
        self.text_parts = []
        self.title = ""
        self.in_title = False
        self.skip_tags = {"script", "style", "noscript", "svg", "path"}
        self.skip_depth = 0

    def handle_starttag(self, tag, attrs):
        if tag in self.skip_tags:
            self.skip_depth += 1
        if tag == "title":
            self.in_title = True

    def handle_endtag(self, tag):
        if tag in self.skip_tags and self.skip_depth > 0:
            self.skip_depth -= 1
        if tag == "title":
            self.in_title = False

    def handle_data(self, data):
        if self.in_title:
            self.title = data.strip()
        elif self.skip_depth == 0:
            text = data.strip()
            if text:
                self.text_parts.append(text)

    def get_text(self):
        return "\n".join(self.text_parts)


def extract_html_content(html):
    """Extract title and text from HTML."""
    parser = HTMLTextExtractor()
    try:
        parser.feed(html)
    except Exception:
        pass
    return parser.title, parser.get_text()


def extract_markdown_title(content):
    """Extract first heading from markdown."""
    for line in content.split("\n"):
        line = line.strip()
        if line.startswith("# "):
            return line[2:].strip()
    return ""


def fetch_url(url):
    """Fetch content from URL, returns (content, content_type)."""
    req = urllib.request.Request(url)
    req.add_header("User-Agent", "mehr-link-provider/1.0")
    req.add_header("Accept", "text/html,text/plain,text/markdown,application/json,*/*")

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            content_type = resp.headers.get("Content-Type", "text/plain")
            charset = "utf-8"
            if "charset=" in content_type:
                charset = content_type.split("charset=")[-1].split(";")[0].strip()
            content = resp.read().decode(charset, errors="replace")
            return content, content_type
    except urllib.error.HTTPError as e:
        raise Exception(f"HTTP {e.code}: {e.reason}")
    except urllib.error.URLError as e:
        raise Exception(f"URL error: {e.reason}")
    except Exception as e:
        raise Exception(f"Fetch error: {str(e)}")


def is_github_issue_url(url):
    """Check if URL is a GitHub issue or PR."""
    return bool(re.match(r"https://github\.com/[^/]+/[^/]+/(issues|pull)/\d+", url))


def fetch_github_issue(url):
    """Fetch GitHub issue via API for better content."""
    # Convert web URL to API URL
    match = re.match(r"https://github\.com/([^/]+)/([^/]+)/(issues|pull)/(\d+)", url)
    if not match:
        return None

    owner, repo, issue_type, number = match.groups()
    api_url = f"https://api.github.com/repos/{owner}/{repo}/issues/{number}"

    req = urllib.request.Request(api_url)
    req.add_header("User-Agent", "mehr-link-provider/1.0")
    req.add_header("Accept", "application/vnd.github+json")

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            title = data.get("title", "")
            body = data.get("body", "") or ""
            return {
                "title": title,
                "description": body,
                "labels": [l.get("name") for l in data.get("labels", [])],
                "status": "closed" if data.get("state") == "closed" else "open",
                "number": number,
                "repo": f"{owner}/{repo}",
                "type": "pr" if issue_type == "pull" else "issue",
            }
    except Exception:
        return None


def extract_key_from_url(url):
    """Extract a meaningful key from URL for branch naming."""
    parsed = urlparse(url)

    # For GitHub issues/PRs, this is handled separately
    if is_github_issue_url(url):
        match = re.match(r"https://github\.com/[^/]+/[^/]+/(issues|pull)/(\d+)", url)
        if match:
            return match.group(2)

    # For other URLs, try to extract meaningful identifier
    path = parsed.path.strip("/")
    if path:
        # Get last path segment, clean it up
        segment = path.split("/")[-1]
        # Remove common extensions
        segment = re.sub(r"\.(html?|php|aspx?|jsp|md|txt)$", "", segment, flags=re.I)
        # Clean up non-alphanumeric chars
        segment = re.sub(r"[^a-zA-Z0-9_-]", "-", segment)
        segment = re.sub(r"-+", "-", segment).strip("-")
        if segment and len(segment) <= 50:
            return segment

    # Fallback to domain
    return parsed.netloc.replace(".", "-")


def is_pastebin_url(url):
    """Check if URL is a Pastebin link."""
    return "pastebin.com" in url and "/raw/" not in url


def convert_pastebin_to_raw(url):
    """Convert Pastebin URL to raw content URL."""
    match = re.search(r"pastebin\.com/(?:raw/)?([a-zA-Z0-9]+)", url)
    if match:
        paste_id = match.group(1)
        return f"https://pastebin.com/raw/{paste_id}"
    return url


def generate_id(url):
    """Generate a short ID from URL."""
    return hashlib.sha256(url.encode()).hexdigest()[:12]


def handle_init(params):
    """Initialize the plugin."""
    return {"capabilities": ["read", "snapshot"]}


def handle_match(params):
    """Check if input matches our scheme."""
    inp = params.get("input", "")
    # Match link: or url: prefix
    if inp.startswith("link:") or inp.startswith("url:"):
        return {"matches": True}
    # Also match raw http:// or https:// URLs
    if inp.startswith("http://") or inp.startswith("https://"):
        return {"matches": True}
    return {"matches": False}


def handle_parse(params):
    """Parse input to extract URL."""
    inp = params.get("input", "")

    # Strip scheme prefix
    for prefix in ["link:", "url:"]:
        if inp.startswith(prefix):
            inp = inp[len(prefix) :]
            break

    # Validate URL
    parsed = urlparse(inp)
    if not parsed.scheme or not parsed.netloc:
        return {"error": f"Invalid URL: {inp}"}

    return {"id": inp}


def handle_fetch(params):
    """Fetch content from URL and return WorkUnit."""
    url = params.get("id", "")

    if not url:
        raise Exception("No URL provided")

    title = ""
    description = ""
    labels = []
    status = "open"
    external_key = ""
    task_type = "task"

    # Try GitHub API for issues/PRs
    if is_github_issue_url(url):
        gh_data = fetch_github_issue(url)
        if gh_data:
            title = gh_data["title"]
            description = gh_data["description"]
            labels = gh_data["labels"]
            status = gh_data["status"]
            external_key = gh_data["number"]
            task_type = gh_data["type"]  # "issue" or "pr"

    # If not GitHub or GitHub fetch failed, do regular HTTP fetch
    if not title:
        # Handle Pastebin specially
        fetch_url_to_use = url
        if is_pastebin_url(url):
            fetch_url_to_use = convert_pastebin_to_raw(url)

        content, content_type = fetch_url(fetch_url_to_use)

        if "text/html" in content_type:
            title, description = extract_html_content(content)
        elif "text/markdown" in content_type or url.endswith(".md"):
            title = extract_markdown_title(content)
            description = content
        elif "application/json" in content_type:
            title = "JSON Content"
            try:
                description = json.dumps(json.loads(content), indent=2)
            except Exception:
                description = content
        else:
            # Plain text or other
            title = extract_markdown_title(content) or "Linked Content"
            description = content

    # Fallback title from URL
    if not title:
        parsed = urlparse(url)
        title = parsed.path.split("/")[-1] or parsed.netloc

    # Extract external key from URL if not already set
    if not external_key:
        external_key = extract_key_from_url(url)

    return {
        "id": generate_id(url),
        "externalId": url,
        "externalKey": external_key,
        "taskType": task_type,
        "provider": "link",
        "title": title,
        "description": description,
        "status": status,
        "priority": 3,  # Normal priority
        "labels": labels,
        "source": {"reference": url},
    }


def handle_snapshot(params):
    """Return raw fetched content for storage."""
    url = params.get("id", "")

    if not url:
        raise Exception("No URL provided")

    # Handle Pastebin specially
    fetch_url_to_use = url
    if is_pastebin_url(url):
        fetch_url_to_use = convert_pastebin_to_raw(url)

    content, _ = fetch_url(fetch_url_to_use)

    return {"content": content}


def handle_request(request):
    """Route request to appropriate handler."""
    method = request.get("method", "")
    params = request.get("params", {})

    handlers = {
        "provider.init": handle_init,
        "provider.match": handle_match,
        "provider.parse": handle_parse,
        "provider.fetch": handle_fetch,
        "provider.snapshot": handle_snapshot,
        "shutdown": lambda p: {},
    }

    if method not in handlers:
        return None, {"code": -32601, "message": f"Method not found: {method}"}

    try:
        result = handlers[method](params)
        return result, None
    except Exception as e:
        return None, {"code": -32000, "message": str(e)}


def main():
    """Main loop: read JSON-RPC from stdin, write to stdout."""
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue

        try:
            request = json.loads(line)
        except json.JSONDecodeError:
            continue

        result, error = handle_request(request)

        response = {"jsonrpc": "2.0", "id": request.get("id")}
        if error:
            response["error"] = error
        else:
            response["result"] = result

        print(json.dumps(response), flush=True)


if __name__ == "__main__":
    main()
