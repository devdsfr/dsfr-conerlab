/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./src/**/*.{html,ts}"],
  theme: {
    extend: {
      colors: {
        cornerlab: {
          bg: '#0f172a',
          surface: '#111827',
          primary: '#22c55e',
          accent: '#38bdf8',
        },
      },
    },
  },
  plugins: [],
  corePlugins: {
    preflight: false, // evita conflito com o CSS base do Angular Material
  },
};
