package inbox

import "strings"

// MatchReplies keeps only the inbox messages that look like replies to our own
// outreach: messages whose sender is an address we emailed (from send history).
// Thread-reference matching could be added later, but sender-match already
// captures the common case (someone replying to our cold email). Newest-first
// order from FetchRecent is preserved; duplicates by sender are collapsed to the
// most recent message so one reply per contact surfaces.
func MatchReplies(msgs []Message, sentTo map[string]bool) []Message {
	if len(sentTo) == 0 {
		return []Message{}
	}
	seen := map[string]bool{}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		from := strings.ToLower(strings.TrimSpace(m.FromEmail))
		if from == "" || !sentTo[from] {
			continue
		}
		if seen[from] {
			continue // already kept the newer message from this sender
		}
		seen[from] = true
		out = append(out, m)
	}
	return out
}
