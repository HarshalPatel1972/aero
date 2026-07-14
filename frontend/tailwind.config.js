/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Aero Brand Palette (Neo-Brutalist)
        aero: {
          cyan: '#C85A27',      // Primary neo-brutalist orange/red
          'cyan-dim': '#A6481E', // Dimmed hover state
        },
        void: {
          black: '#F4F1EA',     // Main background (light cream)
          surface: '#ffffff',   // Solid white for panels
          elevated: '#ffffff',  // Solid white
          border: '#111111',    // Hard black borders
        },
        main: '#111111',        // Main text color
      },
      fontFamily: {
        sans: ['Anton', 'sans-serif'],
        mono: ['Space Mono', 'monospace'],
      },
      animation: {
        'breathe': 'breathe 4s ease-in-out infinite',
        'glow-pulse': 'glowPulse 2s ease-in-out infinite',
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
      },
      keyframes: {
        breathe: {
          '0%, 100%': { opacity: '0.8', transform: 'scale(1)' },
          '50%': { opacity: '1', transform: 'scale(1.02)' },
        },
        glowPulse: {
          '0%, 100%': { boxShadow: '0 0 20px rgba(0, 229, 255, 0.3)' },
          '50%': { boxShadow: '0 0 40px rgba(0, 229, 255, 0.5)' },
        },
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
      },
      backdropBlur: {
        xs: '2px',
        glass: '12px',
      },
      boxShadow: {
        'glow-cyan': '4px 4px 0 0 #111111',
        'glow-cyan-lg': '8px 8px 0 0 #111111',
      },
    },
  },
  plugins: [],
}
