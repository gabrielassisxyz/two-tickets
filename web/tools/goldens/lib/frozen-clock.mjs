// Upstream derives request parameters from the current date (the upcoming
// sliders, and the default end of every one-sided date range). The clock is
// frozen at the day the fixtures were recorded so those parameters, and with
// them the fixture each request matches, are the same on every run.
export const FROZEN_NOW = Date.parse('2026-11-24T12:00:00.000Z');

process.env.TZ = 'UTC';

const RealDate = Date;

class FrozenDate extends RealDate {
  constructor(...args) {
    if (args.length === 0) {
      super(FROZEN_NOW);
    } else {
      super(...args);
    }
  }

  static now() {
    return FROZEN_NOW;
  }
}

globalThis.Date = FrozenDate;
