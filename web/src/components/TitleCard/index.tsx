import { MediaStatus } from '@app/compat/constants/media';
import type { MediaType } from '@app/compat/models/Search';
import CachedImage from '@app/components/Common/CachedImage';
import StatusBadgeMini from '@app/components/Common/StatusBadgeMini';
import ErrorCard from '@app/components/TitleCard/ErrorCard';
import Placeholder from '@app/components/TitleCard/Placeholder';
import QuickActions from '@app/components/TitleCard/QuickActions';
import { useIsTouch } from '@app/hooks/useIsTouch';
import globalMessages from '@app/i18n/globalMessages';
import { withProperties } from '@app/utils/typeHelpers';
import { Transition } from '@headlessui/react';
import Link from 'next/link';
import { Fragment, useEffect, useState } from 'react';
import { useInView } from 'react-intersection-observer';
import { useIntl } from 'react-intl';

interface TitleCardProps {
  id: number;
  image?: string;
  summary?: string;
  year?: string;
  title: string;
  userScore?: number;
  mediaType: MediaType;
  status?: MediaStatus;
  canExpand?: boolean;
  inProgress?: boolean;
  isAddedToWatchlist?: number | boolean;
  mutateParent?: () => void;
  // Off where the card belongs to someone else's list, which is read-only.
  showQuickActions?: boolean;
}

const TitleCard = ({
  id,
  image,
  summary,
  year,
  title,
  status,
  mediaType,
  isAddedToWatchlist = false,
  inProgress = false,
  canExpand = false,
  mutateParent,
  showQuickActions = true,
}: TitleCardProps) => {
  const isTouch = useIsTouch();
  const intl = useIntl();
  const [currentStatus, setCurrentStatus] = useState(status);
  const [showDetail, setShowDetail] = useState(false);
  // A card reads its library state only once it is on screen, so a page of
  // sliders does not ask about every title it holds at once.
  const { ref: inViewRef, inView } = useInView({ triggerOnce: true });

  // Just to get the year from the date
  if (year) {
    year = year.slice(0, 4);
  }

  useEffect(() => {
    setCurrentStatus(status);
  }, [status]);

  return (
    <div
      className={canExpand ? 'w-full' : 'w-36 sm:w-36 md:w-44'}
      data-testid="title-card"
      data-media-type={mediaType}
      data-tmdb-id={id}
      ref={inViewRef}
    >
      <div
        className={`relative transform-gpu cursor-default overflow-hidden rounded-xl bg-gray-800 bg-cover outline-none ring-1 transition duration-300 ${
          showDetail
            ? 'scale-105 shadow-lg ring-gray-500'
            : 'scale-100 shadow ring-gray-700'
        }`}
        style={{
          paddingBottom: '150%',
        }}
        onMouseEnter={() => {
          if (!isTouch) {
            setShowDetail(true);
          }
        }}
        onMouseLeave={() => setShowDetail(false)}
        onClick={() => setShowDetail(true)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            setShowDetail(true);
          }
        }}
        role="link"
        tabIndex={0}
      >
        <div className="absolute inset-0 h-full w-full overflow-hidden">
          <CachedImage
            type="tmdb"
            className="absolute inset-0 h-full w-full"
            alt=""
            src={
              image
                ? `https://image.tmdb.org/t/p/w300_and_h450_face${image}`
                : `/images/seerr_poster_not_found_logo_top.png`
            }
            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
            fill
          />
          <div className="absolute left-0 right-0 flex items-center justify-between p-2">
            <div
              className={`pointer-events-none z-40 self-start rounded-full border shadow-md ${
                mediaType === 'movie' || mediaType === 'collection'
                  ? 'border-blue-500 bg-blue-600/80'
                  : 'border-purple-600 bg-purple-600/80'
              }`}
            >
              <div className="flex h-4 items-center px-2 py-2 text-center text-xs font-medium uppercase tracking-wider text-white sm:h-5">
                {mediaType === 'movie'
                  ? intl.formatMessage(globalMessages.movie)
                  : mediaType === 'collection'
                    ? intl.formatMessage(globalMessages.collection)
                    : intl.formatMessage(globalMessages.tvshow)}
              </div>
            </div>
            {currentStatus && currentStatus !== MediaStatus.UNKNOWN && (
              <div className="flex flex-col items-center gap-1">
                <div className="pointer-events-none z-40 flex">
                  <StatusBadgeMini
                    status={currentStatus}
                    inProgress={inProgress}
                    shrink
                  />
                </div>
              </div>
            )}
          </div>
          <Transition
            as={Fragment}
            show={!image || showDetail}
            enter="transition-opacity"
            enterFrom="opacity-0"
            enterTo="opacity-100"
            leave="transition-opacity"
            leaveFrom="opacity-100"
            leaveTo="opacity-0"
          >
            <div className="absolute inset-0 overflow-hidden rounded-xl">
              <Link
                href={
                  mediaType === 'movie'
                    ? `/movie/${id}`
                    : mediaType === 'collection'
                      ? `/collection/${id}`
                      : `/tv/${id}`
                }
                className="absolute inset-0 h-full w-full cursor-pointer overflow-hidden text-left"
                style={{
                  background:
                    'linear-gradient(180deg, rgba(45, 55, 72, 0.4) 0%, rgba(45, 55, 72, 0.9) 100%)',
                }}
              >
                <div className="flex h-full w-full items-end">
                  <div className="px-2 pb-14 text-white">
                    {year && <div className="text-sm font-medium">{year}</div>}

                    <h1
                      className="whitespace-normal text-xl font-bold leading-tight"
                      style={{
                        WebkitLineClamp: 3,
                        display: '-webkit-box',
                        overflow: 'hidden',
                        WebkitBoxOrient: 'vertical',
                        wordBreak: 'break-word',
                      }}
                      data-testid="title-card-title"
                    >
                      {title}
                    </h1>
                    <div
                      className="whitespace-normal text-xs"
                      style={{
                        WebkitLineClamp: 5,
                        display: '-webkit-box',
                        overflow: 'hidden',
                        WebkitBoxOrient: 'vertical',
                        wordBreak: 'break-word',
                      }}
                    >
                      {summary}
                    </div>
                  </div>
                </div>
              </Link>
            </div>
          </Transition>
          {showQuickActions &&
            (mediaType === 'movie' || mediaType === 'tv') && (
              <QuickActions
                mediaType={mediaType}
                tmdbId={id}
                title={title}
                isAddedToWatchlist={!!isAddedToWatchlist}
                inView={inView}
                mutateParent={mutateParent}
              />
            )}
        </div>
      </div>
    </div>
  );
};

export default withProperties(TitleCard, { Placeholder, ErrorCard });
