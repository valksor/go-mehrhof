import { useState, useCallback, type ReactNode } from 'react'
import Markdown from 'react-markdown'
import { COPY_FEEDBACK_MS } from '../lib/constants'
import { ScreenshotBadge } from './ScreenshotBadge'

interface ChatMessageContentProps {
  content: string
  isUser: boolean
}

function CodeBlock({ children, language, isUser }: { children: string; language?: string; isUser: boolean }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(() => {
    void navigator.clipboard.writeText(children).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), COPY_FEEDBACK_MS)
    })
  }, [children])

  return (
    <div className="my-2 -mx-1 relative group">
      <div className="flex items-center justify-between mb-1">
        {language && <span className="text-[10px] uppercase tracking-wide opacity-60">{language}</span>}
        <button
          type="button"
          onClick={handleCopy}
          className={`text-[10px] px-1.5 py-0.5 rounded transition-opacity ${
            isUser ? 'bg-primary-content/20 hover:bg-primary-content/30' : 'bg-base-300 hover:bg-base-content/20'
          } opacity-0 group-hover:opacity-100 ${copied ? 'opacity-100' : ''} ml-auto`}
          aria-label="Copy code"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
      <pre
        className={`p-2 rounded text-xs overflow-x-auto ${
          isUser ? 'bg-primary-content/10' : 'bg-neutral text-neutral-content'
        }`}
      >
        <code>{children}</code>
      </pre>
    </div>
  )
}

export function ChatMessageContent({ content, isUser }: ChatMessageContentProps) {
  // Pre-process: extract @mentions and screenshot refs before markdown parsing
  // We split on @mentions so they render as badges, then pass non-mention segments through markdown
  const mentionRegex = /@(screenshot-[a-zA-Z0-9]+|[\w./-]+\.\w+|[\w/-]+)/g
  const segments: { type: 'text' | 'file' | 'screenshot'; value: string }[] = []
  let lastIndex = 0
  let match

  while ((match = mentionRegex.exec(content)) !== null) {
    if (match.index > lastIndex) {
      segments.push({ type: 'text', value: content.slice(lastIndex, match.index) })
    }
    const mention = match[1]
    if (mention.startsWith('screenshot-')) {
      segments.push({ type: 'screenshot', value: mention.replace('screenshot-', '') })
    } else {
      segments.push({ type: 'file', value: mention })
    }
    lastIndex = mentionRegex.lastIndex
  }
  if (lastIndex < content.length) {
    segments.push({ type: 'text', value: content.slice(lastIndex) })
  }

  return (
    <div className="text-sm break-words">
      {segments.map((segment, i) => {
        if (segment.type === 'screenshot') {
          return <ScreenshotBadge key={`ss-${i}`} screenshotId={segment.value} />
        }
        if (segment.type === 'file') {
          return (
            <span
              key={`file-${i}`}
              className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-mono ${
                isUser ? 'bg-primary-content/20' : 'bg-primary/20 text-primary'
              }`}
              title={segment.value}
            >
              {segment.value.split('/').pop()}
            </span>
          )
        }
        // Render text segments through react-markdown
        return <MarkdownSegment key={`md-${i}`} text={segment.value} isUser={isUser} />
      })}
    </div>
  )
}

function MarkdownSegment({ text, isUser }: { text: string; isUser: boolean }) {
  return (
    <Markdown
      components={{
        // Override code blocks to add copy button
        pre({ children }) {
          return <>{children}</>
        },
        code({ children, className }) {
          const match = className?.match(/language-(\w+)/)
          const codeString = extractText(children)

          // Block-level code (has language class or is inside pre)
          if (match || (className && className.includes('language-'))) {
            return (
              <CodeBlock language={match?.[1]} isUser={isUser}>
                {codeString.trim()}
              </CodeBlock>
            )
          }

          // Check if this is a multi-line code block without a language
          if (codeString.includes('\n')) {
            return <CodeBlock isUser={isUser}>{codeString.trim()}</CodeBlock>
          }

          // Inline code
          return (
            <code className={`px-1 rounded text-xs ${isUser ? 'bg-primary-content/10' : 'bg-base-300'}`}>
              {children}
            </code>
          )
        },
        // Style paragraphs without extra margin for chat context
        p({ children }) {
          return <p className="my-1 first:mt-0 last:mb-0">{children}</p>
        },
        // Style lists
        ul({ children }) {
          return <ul className="list-disc list-inside my-1">{children}</ul>
        },
        ol({ children }) {
          return <ol className="list-decimal list-inside my-1">{children}</ol>
        },
        // Style links
        a({ href, children }) {
          return (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className={isUser ? 'underline' : 'link link-primary'}
            >
              {children}
            </a>
          )
        },
        // Style headings compactly for chat
        h1({ children }) {
          return <p className="font-bold text-base my-1">{children}</p>
        },
        h2({ children }) {
          return <p className="font-bold text-sm my-1">{children}</p>
        },
        h3({ children }) {
          return <p className="font-semibold text-sm my-1">{children}</p>
        },
        // Bold / italic
        strong({ children }) {
          return <strong className="font-bold">{children}</strong>
        },
        em({ children }) {
          return <em className="italic">{children}</em>
        },
        // Blockquote
        blockquote({ children }) {
          return (
            <blockquote
              className={`border-l-2 pl-2 my-1 opacity-80 ${
                isUser ? 'border-primary-content/40' : 'border-base-content/40'
              }`}
            >
              {children}
            </blockquote>
          )
        },
        // Horizontal rule
        hr() {
          return <hr className="my-2 border-base-content/20" />
        },
      }}
    >
      {text}
    </Markdown>
  )
}

/** Recursively extract text content from React children */
function extractText(children: ReactNode): string {
  if (typeof children === 'string') return children
  if (typeof children === 'number') return String(children)
  if (Array.isArray(children)) return children.map(extractText).join('')
  if (children && typeof children === 'object' && 'props' in children) {
    return extractText((children as { props: { children?: ReactNode } }).props.children)
  }
  return ''
}
