package commands

import (
	"testing"
)

// badShape is a JSON value (a plain string) that the commands' result structs
// cannot unmarshal into, so the parse/unmarshal error branch is exercised. The
// RPC call itself succeeds; only the decode of an unexpected response shape
// fails — which the command must surface as an error rather than swallow.
const badShape = "unexpected-scalar-response"

// TestWorktreeCommands_ParseErrors3 drives the json.Unmarshal failure branch of
// worktree-socket commands by returning a scalar where an object is expected.
func TestWorktreeCommands_ParseErrors3(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"queueadd", "queue.add", func() error { return runQueueAdd(queueAddCmd, []string{"src"}) }},
		{"queueremove", "queue.remove", func() error { return runQueueRemove(queueRemoveCmd, []string{"q1"}) }},
		{"queuereorder", "queue.reorder", func() error { return runQueueReorder(queueReorderCmd, []string{"q1", "1"}) }},
		{"screenshotdelete", "screenshots.delete", func() error {
			return runScreenshotsDelete(screenshotsDeleteCmd, []string{"s1"})
		}},
		{"forkcreate", "fork.create", func() error { return runForkCreate(nil, []string{"l"}) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubWorktreeSocket(t)
			stub.SetResponse(c.method, badShape)

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected parse error for malformed %s response", c.name, c.method)
			}
		})
	}
}

// TestGlobalCommands_ParseErrors3 drives the json.Unmarshal failure branch of
// global-socket commands.
func TestGlobalCommands_ParseErrors3(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"cataloglist", "catalog.list", func() error {
			setBoolPtr(t, &catalogListJSON, false)

			return runCatalogList(nil, nil)
		}},
		{"browse", "browse", func() error { return runBrowse(BrowseCmd, nil) }},
		{"grouplist", "taskgroup.list", func() error {
			setBoolPtr(t, &groupListJSON, false)

			return runGroupList(nil, nil)
		}},
		{"statsall", "tasks.list", func() error { return runStatsAll() }},
		{"chatsend", "chat.send", func() error { return runChatSend(chatSendCmd, []string{"hi"}) }},
		{"chatstop", "chat.stop", func() error { return runChatStop(chatStopCmd, nil) }},
		{"chatclear", "chat.clear", func() error { return runChatClear(chatClearCmd, nil) }},
		{"explain", "chat.send", func() error { return runExplain(ExplainCmd, nil) }},
		{"showallstatus", "tasks.list", func() error {
			setBoolPtr(t, &statusJSON, false)

			return showAllStatus()
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubGlobalSocket(t)
			stub.SetResponse(c.method, badShape)

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected parse error for malformed %s response", c.name, c.method)
			}
		})
	}
}
