/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Aero Brand Palette
        aero: {
          cyan: '#00E5FF',      // Bioluminescent Cyan - Primary action
          'cyan-dim': '#00B4D8', // Dimmed cyan for hover states
        },
        void: {
          black: '#050505',     // Vantablack - Deep void
          surface: '#121212',   // Glass Charcoal
          elevated: '#1a1a1a',  // Elevated surface
          border: '#2a2a2a',    // Subtle borders
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'Roboto Mono', 'monospace'],
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
        'glow-cyan': '0 0 20px rgba(0, 229, 255, 0.4)',
        'glow-cyan-lg': '0 0 40px rgba(0, 229, 255, 0.6)',
      },
    },
  },
  plugins: [],
}
