package notify

// FormatSlackPayload converts a Payload into a Slack Block Kit message structure.
func FormatSlackPayload(p Payload) map[string]any {
	fields := []map[string]any{
		{
			slackKeyType: slackTypeMarkdown,
			slackKeyText: "*State:* " + p.State,
		},
	}

	if p.PreviousState != "" && p.PreviousState != p.State {
		fields = append(fields, map[string]any{
			slackKeyType: slackTypeMarkdown,
			slackKeyText: "*Previous State:* " + p.PreviousState,
		})
	}

	if p.ProjectPath != "" {
		fields = append(fields, map[string]any{
			slackKeyType: slackTypeMarkdown,
			slackKeyText: "*Project:* " + p.ProjectPath,
		})
	}

	blocks := []map[string]any{
		{
			slackKeyType: "section",
			slackKeyText: map[string]any{
				slackKeyType: slackTypeMarkdown,
				slackKeyText: "*" + p.TaskTitle + "*",
			},
			"fields": fields,
		},
	}

	if p.Error != "" {
		blocks = append(blocks, map[string]any{
			slackKeyType: "context",
			"elements": []map[string]any{
				{
					slackKeyType: slackTypeMarkdown,
					slackKeyText: ":warning: " + p.Error,
				},
			},
		})
	}

	return map[string]any{
		"blocks": blocks,
	}
}
