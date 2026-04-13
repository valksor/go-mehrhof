package notify

// FormatSlackPayload converts a Payload into a Slack Block Kit message structure.
func FormatSlackPayload(p Payload) map[string]any {
	fields := []map[string]any{
		{
			"type": "mrkdwn",
			"text": "*State:* " + p.State,
		},
	}

	if p.PreviousState != "" && p.PreviousState != p.State {
		fields = append(fields, map[string]any{
			"type": "mrkdwn",
			"text": "*Previous State:* " + p.PreviousState,
		})
	}

	if p.ProjectPath != "" {
		fields = append(fields, map[string]any{
			"type": "mrkdwn",
			"text": "*Project:* " + p.ProjectPath,
		})
	}

	blocks := []map[string]any{
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": "*" + p.TaskTitle + "*",
			},
			"fields": fields,
		},
	}

	if p.Error != "" {
		blocks = append(blocks, map[string]any{
			"type": "context",
			"elements": []map[string]any{
				{
					"type": "mrkdwn",
					"text": ":warning: " + p.Error,
				},
			},
		})
	}

	return map[string]any{
		"blocks": blocks,
	}
}
