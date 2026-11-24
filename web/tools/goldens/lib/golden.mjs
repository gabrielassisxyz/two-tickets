// What a golden pins, and how a response is compared with it.
//
// A golden holds the response body as upstream produced it from the fixtures,
// with one exception: the values TMDB changes daily. Under the keys in
// VOLATILE_KEYS the value is replaced by a type marker ("<number>"), so the
// golden records that the field is present and what JSON type it has, never
// the figure itself. compareToGolden applies the same rule from the other
// side: field names, presence and JSON types are compared everywhere, values
// everywhere except under a volatile key.

export const VOLATILE_KEYS = new Set([
  'popularity',
  'voteCount',
  'voteAverage',
]);

const MARKER = /^<(null|boolean|number|string|array|object)>$/;

export const jsonTypeOf = (value) => {
  if (value === null) {
    return 'null';
  }
  if (Array.isArray(value)) {
    return 'array';
  }
  return typeof value;
};

const markerFor = (value) => `<${jsonTypeOf(value)}>`;

const isPlainObject = (value) => jsonTypeOf(value) === 'object';

export const normalizeForGolden = (value) => {
  if (Array.isArray(value)) {
    return value.map(normalizeForGolden);
  }
  if (!isPlainObject(value)) {
    return value;
  }
  const normalized = {};
  for (const [key, child] of Object.entries(value)) {
    normalized[key] = VOLATILE_KEYS.has(key)
      ? markerFor(child)
      : normalizeForGolden(child);
  }
  return normalized;
};

const expectedVolatileType = (golden) => {
  const marker = typeof golden === 'string' ? MARKER.exec(golden) : null;
  return marker ? marker[1] : jsonTypeOf(golden);
};

const compareObjects = (actual, golden, path, differences) => {
  for (const key of Object.keys(golden)) {
    if (!Object.hasOwn(actual, key)) {
      differences.push(`${path}.${key}: missing`);
    }
  }
  for (const key of Object.keys(actual)) {
    if (!Object.hasOwn(golden, key)) {
      differences.push(`${path}.${key}: unexpected field`);
      continue;
    }
    compareAt(actual[key], golden[key], `${path}.${key}`, key, differences);
  }
};

const compareArrays = (actual, golden, path, differences) => {
  if (actual.length !== golden.length) {
    differences.push(
      `${path}: expected ${golden.length} items, got ${actual.length}`
    );
    return;
  }
  actual.forEach((item, index) =>
    compareAt(item, golden[index], `${path}[${index}]`, null, differences)
  );
};

const compareAt = (actual, golden, path, key, differences) => {
  const actualType = jsonTypeOf(actual);
  if (key !== null && VOLATILE_KEYS.has(key)) {
    const expected = expectedVolatileType(golden);
    if (actualType !== expected) {
      differences.push(`${path}: expected ${expected}, got ${actualType}`);
    }
    return;
  }
  const goldenType = jsonTypeOf(golden);
  if (actualType !== goldenType) {
    differences.push(`${path}: expected ${goldenType}, got ${actualType}`);
    return;
  }
  if (goldenType === 'object') {
    compareObjects(actual, golden, path, differences);
  } else if (goldenType === 'array') {
    compareArrays(actual, golden, path, differences);
  } else if (actual !== golden) {
    differences.push(
      `${path}: expected ${JSON.stringify(golden)}, got ${JSON.stringify(actual)}`
    );
  }
};

// Returns every difference as "<path>: <what differs>"; empty means a match.
export const compareToGolden = (actual, golden) => {
  const differences = [];
  compareAt(actual, golden, '$', null, differences);
  return differences;
};

export const serializeJson = (value) => `${JSON.stringify(value, null, 2)}\n`;
