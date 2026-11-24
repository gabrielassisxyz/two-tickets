// Only non-catalog routes are marked deprecated.
const deprecatedRoute = () => (_req, _res, next) => next();

export default deprecatedRoute;
