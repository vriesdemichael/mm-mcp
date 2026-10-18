---
search:
  boost: 0.3
---

# ADR-033: The client decides whether a write runs, and mm-mcp enforces its own guards

Whether a write runs is the MCP client's to decide: its prompt, its permission rules, or its auto mode. mm-mcp does not ask the person by default. What it controls is what it enforces itself, on every call, whoever approved it:

- It writes nothing until `MM_MCP_ALLOW_WRITES` is true (ADR-021).
- A message is checked before it is posted, and each post and edit is marked as written with AI, on by default (ADR-031, ADR-034).
- Each write others see states what it does to them, as numbers mm-mcp counts itself: how many people a post reaches and notifies, and how many characters or posts a call takes away (ADR-031). A missing or wrong number is refused with the real one, and the refusal tells the model to consider whether that is what the person meant before it calls again.
- Each write others see names its places by address, matched exactly (ADR-030).
- Each number is counted again just before the write, so a call made on an outdated reading is refused: members who joined, a reply that arrived, a post edited or a draft typed on since the model read it.

The client's approval sees the tool's name and its arguments, and in Claude Code's auto mode nothing else, so what the write does to others is put in its arguments rather than in a question only some clients show.

`MM_MCP_ELICITATION_OPT_IN`, `false` by default, has mm-mcp ask as well. Each write others see then asks the person through MCP elicitation before it acts: what will be sent, where and under whose name, with one required checkbox and no default, so a client whose approvals are automatic cannot accept a form with no fields. The question is bound to the call's input and to what the call acts on, its fingerprint read again just before it acts, so an answer accepts only what the person was shown. A client that cannot show the question is refused with the error MCP defines for a missing client capability, and nothing is written. A refusal tells the model that either the person declined or their client answered without showing the question, and names `save_draft` and unsetting the setting as the ways forward. While the setting is on, the server's instructions and each such tool's description say the person is asked; otherwise they say mm-mcp does not ask and the client approves the call.

It is a convenience in a client that shows the question, not a safeguard. The answer reaches mm-mcp from the client, and mm-mcp cannot tell the person's answer from the client's. The Claude desktop app's Code tab and its VS Code extension decline the question unseen ([mm-mcp#20](https://github.com/vriesdemichael/mm-mcp/issues/20); upstream [anthropics/claude-code#89858](https://github.com/anthropics/claude-code/issues/89858) and [#96043](https://github.com/anthropics/claude-code/issues/96043)), Codex in Full Access accepts a form without fields on its own, Copilot CLI's Autopilot answers elicitations itself, about half the coding clients support none, and the 2026-07-28 revision lets a client gather the answer "from the user or other sources". Presenting such an answer as the safety mechanism would mislead anyone who does not know MCP well.

`MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE`, `false` by default, marks each write others see with `_meta["anthropic/requiresUserInteraction"]: true`. Claude Code then shows its own prompt on every call, in every permission mode, and no allow rule or "don't ask again" skips it. It is Claude Code's own mark, which no other client reads, so it is documented as a Claude Code feature, not a guarantee. With `MM_MCP_ELICITATION_OPT_IN=true` as well, the server stops at start with what to change: the person would be asked twice for one write.

`MM_MCP_ASK_BEFORE_WRITES` sets nothing. A server started with it set stops and names `MM_MCP_ELICITATION_OPT_IN`, so nobody believes a setting decides who asks when it does not. The server says at start who decides on a write.

Not every write has a person in the loop. An agent that works alone, such as a scheduled report, needs only to be allowed by the client's rules. That is why forcing a human in the loop is a setting of its own: a mark that overrides allow rules makes unattended writing impossible. A change that is the user's alone or gone in seconds never asks, whatever the settings (ADR-021).

The names say what the person gets. "Elicitation opt-in" names the mechanism and that it is chosen, not assumed; "force human in the loop" names the outcome, a person on every call, overriding what the client would otherwise allow; and "in Claude Code" names the one client that enforces it. Each is a yes or no, and the one combination that contradicts itself is refused rather than resolved silently.

## Not chosen

- **Asking by default, as the safety mechanism**: An answer through the protocol is no proof a person saw anything, and in the client that answers unseen nothing could be written at all.
- **A step that has the model check its own plan, or an acknowledgement it sets**: Such as answering ABORT or CONFIRMED, or `target_audience_ack` ([mm-mcp#42](https://github.com/vriesdemichael/mm-mcp/issues/42)): the model grading its own work, where a number mm-mcp counts cannot be set without looking.
- **One setting with three values**: Values naming who asks, such as `mm-mcp` or `client`, read as something to install or connect rather than as a choice.
- **The mark on every write by default**: It overrides allow rules, so no agent could write unattended.
- **The mark while mm-mcp asks too**: The person would be asked twice for one post, wearing out the attention a question depends on.
- **Detect the client and stop asking it**: A client's name says nothing reliable about whether it shows questions, and the same client behaves differently from one mode to another.
- **Start with `MM_MCP_ASK_BEFORE_WRITES` set, ignoring it**: Whoever set it would believe it decides who asks.
