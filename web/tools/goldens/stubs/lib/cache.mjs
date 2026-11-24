// Every lookup misses, so each TMDB call reaches the fixtures and none is
// answered from an earlier case.
const missingStore = {
  get: () => undefined,
  set: () => true,
  del: () => 0,
  getTtl: () => 0,
};

export default {
  getCache: () => ({ data: missingStore }),
};
