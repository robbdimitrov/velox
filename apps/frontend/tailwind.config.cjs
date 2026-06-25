/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        carbon: '#0F0F11',
        panel: '#17171B',
        line: '#2A2A31',
        ink: '#D7D7DE',
        urgency: '#FF3366',
        signal: '#5533FF',
        ok: '#25D28A',
        warn: '#F5B841'
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'monospace']
      },
      borderRadius: {
        ui: '4px'
      }
    }
  },
  daisyui: {
    themes: [
      {
        velox: {
          primary: '#5533FF',
          secondary: '#25D28A',
          accent: '#FF3366',
          neutral: '#17171B',
          'base-100': '#0F0F11',
          'base-200': '#17171B',
          'base-300': '#2A2A31',
          'base-content': '#D7D7DE',
          info: '#7C8CFF',
          success: '#25D28A',
          warning: '#F5B841',
          error: '#FF3366'
        }
      }
    ]
  },
  plugins: [require('daisyui')]
};
