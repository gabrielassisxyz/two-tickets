// eslint-disable-next-line @typescript-eslint/no-require-imports
const defaultTheme = require('tailwindcss/defaultTheme');

// The project's identity, applied here so every kept and new component
// follows it through the palette classes it already uses (gray-*,
// indigo-*, ...), without restyling components one by one.
//
// - Ground: neutral charcoal surfaces at the lightness steps of upstream's
//   gray, so posters, cards and their hairlines keep the contrast they have
//   in Seerr; text is warm off-white, never pure black or pure white.
// - One accent, the yellow of the movie-filtering project, doing one job:
//   what is active, selected or pressable. Every chromatic family upstream
//   uses (indigo, purple, blue, green) maps onto it, so the page stays
//   monochromatic and upstream's indigo-to-purple gradients collapse into
//   one flat colour. Text on an accent fill is `ink`, never white.
// - One muted red for the rare thing that must shout (errors, danger);
//   upstream's amber, orange and yellow warnings map onto it.
const ground = {
  50: '#f5f4f0',
  100: '#e8e6e0',
  200: '#d4d2cb',
  300: '#b8b5ad',
  400: '#8f8c86',
  500: '#71717a',
  600: '#52525b',
  700: '#3f3f46',
  800: '#27272a',
  900: '#18181b',
  950: '#09090b',
};

// 500 is movie-filtering's --accent and 700 its --accent-dim.
const accent = {
  50: '#fdf9ef',
  100: '#faf1da',
  200: '#f5e4b8',
  300: '#f0d89c',
  400: '#efd699',
  500: '#e8c87a',
  600: '#dcbb6c',
  700: '#c4a355',
  800: '#8c7339',
  900: '#5a4a25',
  950: '#2e2614',
};

// The text colour on an accent fill: movie-filtering's ink on its yellow.
const ink = '#0c0c0f';

const signal = {
  50: '#f9f1f1',
  100: '#f0dbdc',
  200: '#e4b9bc',
  300: '#d69498',
  400: '#cb767c',
  500: '#c0595f',
  600: '#a54047',
  700: '#86373c',
  800: '#632c30',
  900: '#442224',
  950: '#281517',
};

// Depth comes from hairlines and surface steps, not from frosted glass:
// every backdrop blur upstream asks for renders unblurred.
const noBlur = Object.fromEntries(
  Object.keys(defaultTheme.blur).map((key) => [key, '0'])
);

/** @type {import('tailwindcss').Config} */
module.exports = {
  mode: 'jit',
  content: [
    './node_modules/@seerr-team/react-tailwindcss-datepicker/dist/index.esm.js',
    './src/pages/**/*.{ts,tsx}',
    './src/components/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      transitionProperty: {
        'max-height': 'max-height',
        width: 'width',
      },
      colors: {
        gray: ground,
        ink,
        indigo: accent,
        purple: accent,
        blue: accent,
        green: accent,
        red: signal,
        orange: signal,
        amber: signal,
        yellow: signal,
      },
      // Small radii: sharply bounded surfaces rather than wide rounded cards.
      borderRadius: {
        md: '0.3125rem',
        lg: '0.375rem',
        xl: '0.5rem',
        '2xl': '0.625rem',
        '3xl': '0.75rem',
      },
      backdropBlur: noBlur,
      // One sans family for titles, interface and body text; titles get
      // their weight from the face, never from a serif display.
      fontFamily: {
        sans: ['DM Sans Variable', ...defaultTheme.fontFamily.sans],
      },
      typography: (theme) => ({
        DEFAULT: {
          css: {
            color: theme('colors.gray.300'),
            a: {
              color: theme('colors.indigo.500'),
              '&:hover': {
                color: theme('colors.indigo.400'),
              },
            },

            h1: {
              color: theme('colors.gray.300'),
            },
            h2: {
              color: theme('colors.gray.300'),
            },
            h3: {
              color: theme('colors.gray.300'),
            },
            h4: {
              color: theme('colors.gray.300'),
            },
            h5: {
              color: theme('colors.gray.300'),
            },
            h6: {
              color: theme('colors.gray.300'),
            },

            strong: {
              color: theme('colors.gray.400'),
            },

            code: {
              color: theme('colors.gray.300'),
            },

            figcaption: {
              color: theme('colors.gray.500'),
            },
          },
        },
      }),
    },
    aspectRatio: {
      auto: 'auto',
      square: '1 / 1',
      video: '16 / 9',
      1: '1',
      2: '2',
      3: '3',
      4: '4',
      5: '5',
      6: '6',
      7: '7',
      8: '8',
      9: '9',
      10: '10',
      11: '11',
      12: '12',
      13: '13',
      14: '14',
      15: '15',
      16: '16',
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
    require('@tailwindcss/aspect-ratio'),
  ],
};
