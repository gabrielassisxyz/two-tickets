// An empty database with one signed-in user: no title is requested, watchlisted
// or blocklisted, so every route answers from TMDB alone. That is the only state
// two-tickets has, since it keeps no request or media tables of Seerr's.
import { User } from '@server/entity/User';
import { Watchlist } from '@server/entity/Watchlist';

export const GOLDENS_USER = {
  id: 1,
  settings: { locale: 'pt-BR' },
  hasPermission: () => true,
};

const repositories = new Map([
  [
    User,
    {
      findOne: async () => GOLDENS_USER,
      count: async () => 1,
    },
  ],
  [
    Watchlist,
    {
      exist: async () => false,
      findAndCount: async () => [[], 0],
    },
  ],
]);

export const getRepository = (entity) => {
  const repository = repositories.get(entity);
  if (!repository) {
    throw new Error(`goldens harness: no repository stub for ${entity?.name}`);
  }
  return repository;
};
