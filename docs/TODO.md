# TODO

## Claude Compatibility

- [x] Emit Anthropic-style SSE event names for Claude streaming responses (`event: message_start`, `event: content_block_delta`, etc.).
- [x] Implement Claude `tool_choice` semantics for `none`, `auto`, `any`, and forced tool names.
- [ ] Make tool bridge output parsing more tolerant by extracting the first JSON object from model text and retrying parse failures once.
- [ ] Preserve `input: {}` on `tool_use` responses even for zero-argument tools.
- [ ] Add provider readiness reporting so `/health` or a new `/ready` endpoint can distinguish process liveness from Gemini cookie/session health.
- [ ] Remove unsafe debug body slicing in Claude request parse errors.
