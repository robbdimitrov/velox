/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        carbon: '#050505',
        panel: '#0B0C10',
        line: '#1F2937',
        ink: '#F8FAFC',
        inkMuted: '#94A3B8',
        urgency: '#EF4444',
        signal: '#FACC15',
        signalHover: '#FDE047',
        ok: '#10B981',
        warn: '#F59E0B'
      },
      fontFamily: {
        sans: ['"Space Grotesk"', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['"Space Mono"', 'ui-monospace', 'monospace']
      },
      borderRadius: {
        ui: '12px',
        card: '20px'
      },
      boxShadow: {
        glow: '0 0 20px rgba(250, 204, 21, 0.4)',
        glass: '0 8px 32px 0 rgba(0, 0, 0, 0.37)'
      }
    }
  },
  daisyui: {
    themes: [
      {
        velox: {
          primary: '#FACC15',
          secondary: '#94A3B8',
          accent: '#38BDF8',
          neutral: '#0B0C10',
          'base-100': '#050505',
          'base-200': '#0B0C10',
          'base-300': '#1F2937',
          'base-content': '#F8FAFC',
          info: '#3B82F6',
          success: '#10B981',
          warning: '#F59E0B',
          error: '#EF4444'
        }
      }
    ]
  },
  plugins: [require('daisyui')]
};
