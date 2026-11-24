// The golden-comparison helper: presence, JSON type, and the volatile fields
// whose values are ignored but whose presence and type are not.
// Run with: node --test lib/
import { strict as assert } from 'node:assert';
import { describe, it } from 'node:test';
import { compareToGolden, normalizeForGolden } from './golden.mjs';

const movie = {
  id: 603,
  mediaType: 'movie',
  title: 'Matrix',
  posterPath: '/poster.jpg',
  backdropPath: null,
  popularity: 41.5,
  voteCount: 27000,
  voteAverage: 8.2,
  releases: { results: [{ iso_3166_1: 'BR', release_dates: [] }] },
};

const golden = normalizeForGolden({ page: 1, results: [movie] });

describe('normalizeForGolden', () => {
  it('replaces the values of the volatile fields with their JSON type', () => {
    const [result] = golden.results;
    assert.equal(result.popularity, '<number>');
    assert.equal(result.voteCount, '<number>');
    assert.equal(result.voteAverage, '<number>');
  });

  it('keeps every other value, at any depth', () => {
    const [result] = golden.results;
    assert.equal(result.title, 'Matrix');
    assert.equal(result.backdropPath, null);
    assert.deepEqual(result.releases, movie.releases);
  });

  it('records a null volatile value as null, not as a number', () => {
    assert.deepEqual(normalizeForGolden({ popularity: null }), {
      popularity: '<null>',
    });
  });
});

describe('compareToGolden', () => {
  const withMovie = (changes) => ({
    page: 1,
    results: [{ ...movie, ...changes }],
  });

  it('matches the response the golden was made from', () => {
    assert.deepEqual(compareToGolden(withMovie({}), golden), []);
  });

  it('ignores the values of popularity, voteCount and voteAverage', () => {
    const drifted = withMovie({
      popularity: 3.1,
      voteCount: 1,
      voteAverage: 0,
    });
    assert.deepEqual(compareToGolden(drifted, golden), []);
  });

  it('reports a volatile field that is missing', () => {
    const response = withMovie({});
    delete response.results[0].voteCount;
    assert.deepEqual(compareToGolden(response, golden), [
      '$.results[0].voteCount: missing',
    ]);
  });

  it('reports a volatile field whose type changed', () => {
    assert.deepEqual(
      compareToGolden(withMovie({ voteAverage: '8.2' }), golden),
      ['$.results[0].voteAverage: expected number, got string']
    );
  });

  it('reports a field that is missing and one that is unexpected', () => {
    const response = withMovie({ posterUrl: '/poster.jpg' });
    delete response.results[0].posterPath;
    assert.deepEqual(compareToGolden(response, golden), [
      '$.results[0].posterPath: missing',
      '$.results[0].posterUrl: unexpected field',
    ]);
  });

  it('tells null from absent', () => {
    const response = withMovie({});
    delete response.results[0].backdropPath;
    assert.deepEqual(compareToGolden(response, golden), [
      '$.results[0].backdropPath: missing',
    ]);
  });

  it('reports a changed JSON type outside the volatile fields', () => {
    assert.deepEqual(
      compareToGolden(withMovie({ backdropPath: '/b.jpg' }), golden),
      ['$.results[0].backdropPath: expected null, got string']
    );
  });

  it('compares values outside the volatile fields', () => {
    assert.deepEqual(
      compareToGolden(withMovie({ title: 'The Matrix' }), golden),
      ['$.results[0].title: expected "Matrix", got "The Matrix"']
    );
  });

  it('reports an array of a different length', () => {
    assert.deepEqual(compareToGolden({ page: 1, results: [] }, golden), [
      '$.results: expected 1 items, got 0',
    ]);
  });

  it('keeps snake_case keys nested in camelCase objects exact', () => {
    const response = withMovie({
      releases: { results: [{ iso3166_1: 'BR', release_dates: [] }] },
    });
    assert.deepEqual(compareToGolden(response, golden), [
      '$.results[0].releases.results[0].iso_3166_1: missing',
      '$.results[0].releases.results[0].iso3166_1: unexpected field',
    ]);
  });
});
