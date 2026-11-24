// Bundles the harness entry together with the pinned upstream server modules.
// Each upstream module is either taken as is from the fetched commit (the mappers,
// the TMDB client and the catalog routes) or replaced by a stub under stubs/
// (database, settings, logger, cache, and every route family this project does
// not port). An upstream import that is neither fails the build, so a module can
// never be pulled in, or stubbed, without being named here.
import { build } from 'esbuild';
import { existsSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const toolDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const stubDir = join(toolDir, 'stubs');

export const UPSTREAM_MODULES = new Set([
  'api/externalapi',
  'api/metadata',
  'api/themoviedb/constants',
  'api/themoviedb/index',
  'constants/media',
  'lib/permissions',
  'lib/search',
  'middleware/auth',
  'models/Collection',
  'models/Movie',
  'models/Person',
  'models/Search',
  'models/Tv',
  'models/common',
  'routes/collection',
  'routes/discover',
  'routes/index',
  'routes/movie',
  'routes/person',
  'routes/search',
  'routes/tv',
  'utils/typeHelpers',
]);

const findSource = (base) =>
  [`${base}.ts`, join(base, 'index.ts')].find((candidate) =>
    existsSync(candidate)
  );

const moduleIdOf = (serverDir, file) =>
  relative(serverDir, file).replace(/\.ts$/, '');

const upstreamPlugin = (serverDir) => ({
  name: 'seerr-upstream',
  setup(pluginBuild) {
    const resolveUpstream = (base, importer) => {
      const file = findSource(base);
      if (!file) {
        throw new Error(`upstream module not found: ${base} (from ${importer})`);
      }
      const id = moduleIdOf(serverDir, file);
      const stub = join(stubDir, `${id.replace(/\/index$/, '')}.mjs`);
      if (existsSync(stub)) {
        return { path: stub };
      }
      if (UPSTREAM_MODULES.has(id)) {
        return { path: file };
      }
      return {
        errors: [
          {
            text: `upstream module ${id} is neither listed in UPSTREAM_MODULES nor stubbed under stubs/ (imported from ${importer})`,
          },
        ],
      };
    };

    pluginBuild.onResolve({ filter: /^@server\// }, (args) =>
      resolveUpstream(
        join(serverDir, args.path.slice('@server/'.length)),
        args.importer
      )
    );
    pluginBuild.onResolve({ filter: /^\.\.?\// }, (args) => {
      if (!args.importer.startsWith(serverDir)) {
        return undefined;
      }
      return resolveUpstream(join(args.resolveDir, args.path), args.importer);
    });
    // Every package import stays external and resolves from this tool's own
    // node_modules at run time, at the versions upstream pins.
    pluginBuild.onResolve({ filter: /^[^./]/ }, (args) => {
      if (args.path.startsWith('@server/')) {
        return undefined;
      }
      return { path: args.path, external: true };
    });
  },
});

export const bundleHarness = async ({ upstreamDir, outfile }) => {
  const serverDir = join(upstreamDir, 'server');
  await build({
    entryPoints: [join(toolDir, 'lib', 'harness-entry.mjs')],
    outfile,
    bundle: true,
    platform: 'node',
    // Upstream's server compiles to CommonJS; bundling to the same format keeps
    // its default-import interop with CommonJS packages unchanged.
    format: 'cjs',
    target: 'node22',
    logLevel: 'error',
    plugins: [upstreamPlugin(serverDir)],
  });
  return outfile;
};
