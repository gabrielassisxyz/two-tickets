// Upstream logs through winston to files; the harness discards it.
const discard = () => undefined;

export default {
  debug: discard,
  info: discard,
  warn: discard,
  error: discard,
};
