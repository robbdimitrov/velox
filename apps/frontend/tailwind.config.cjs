/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        carbon: '#09090E',
        panel: '#15151A',
        line: '#272730',
        ink: '#F3F4F6',
        inkMuted: '#9CA3AF',
        urgency: '#FF2A5F',
        signal: '#6D28D9',
        signalHover: '#7C3AED',
        ok: '#10B981',
        warn: '#F59E0B'
      },
      fontFamily: {
        sans: ['Outfit', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace']
      },
      borderRadius: {
        ui: '12px',
        card: '20px'
      },
      boxShadow: {
        glow: '0 0 20px rgba(109, 40, 217, 0.4)',
        glass: '0 8px 32px 0 rgba(0, 0, 0, 0.37)'
      }
    }
  },
  daisyui: {
    themes: [
      {
        velox: {
          primary: '#7C3AED',
          secondary: '#10B981',
          accent: '#FF2A5F',
          neutral: '#15151A',
          'base-100': '#09090E',
          'base-200': '#15151A',
          'base-300': '#272730',
          'base-content': '#F3F4F6',
          info: '#3B82F6',
          success: '#10B981',
          warning: '#F59E0B',
          error: '#FF2A5F'
        }
      }
    ]
  },
  plugins: [require('daisyui')]
};
