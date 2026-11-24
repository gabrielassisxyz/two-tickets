import type { LibraryCard } from '@app/components/Library/libraryTypes';
import TitleCard from '@app/components/TitleCard';

interface LibraryCardForProps {
  item: LibraryCard;
  isAddedToWatchlist?: boolean;
  canExpand?: boolean;
  showQuickActions?: boolean;
  mutateParent?: () => void;
}

// A title card for a hydrated library item. `result` is the shared search
// result shape, so the card renders exactly as it does in search; the
// watchlist star starts from what the caller knows and the card refines it
// from the title's own library state once it is on screen.
const LibraryCardFor = ({
  item,
  isAddedToWatchlist = false,
  canExpand,
  showQuickActions,
  mutateParent,
}: LibraryCardForProps) => {
  const result = item.result;
  const isMovie = result.mediaType === 'movie';
  return (
    <TitleCard
      id={item.tmdbId}
      mediaType={item.mediaType}
      image={result.posterPath}
      summary={result.overview}
      title={isMovie ? result.title : result.name}
      year={isMovie ? result.releaseDate : result.firstAirDate}
      userScore={result.voteAverage}
      isAddedToWatchlist={
        isAddedToWatchlist || (result.mediaInfo?.watchlists?.length ?? 0) > 0
      }
      canExpand={canExpand}
      showQuickActions={showQuickActions}
      mutateParent={mutateParent}
    />
  );
};

export default LibraryCardFor;
