/**
 * Toast.tsx - Glass Notification System
 * Term-Phase 9: "Glass & Air" Aesthetic
 *
 * Features:
 * - Glassmorphism styling (backdrop blur + subtle borders)
 * - Framer Motion animations (slide + fade)
 * - Responsive positioning (bottom-right desktop, top-center mobile)
 * - Context-based API for global access
 */

import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

type ToastType = 'success' | 'error' | 'info' | 'warning';

interface Toast {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  duration?: number;
}

interface ToastContextValue {
  toasts: Toast[];
  success: (title: string, message?: string) => void;
  error: (title: string, message?: string) => void;
  info: (title: string, message?: string) => void;
  warning: (title: string, message?: string) => void;
  dismiss: (id: string) => void;
  dismissAll: () => void;
}

// ═══════════════════════════════════════════════════════════════
// CONTEXT
// ═══════════════════════════════════════════════════════════════

const ToastContext = createContext<ToastContextValue | null>(null);

export const useToast = () => {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within ToastProvider');
  }
  return context;
};

// ═══════════════════════════════════════════════════════════════
// ICONS
// ═══════════════════════════════════════════════════════════════

const icons: Record<ToastType, React.ReactNode> = {
  success: (
    <svg className="w-5 h-5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
    </svg>
  ),
  error: (
    <svg className="w-5 h-5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
    </svg>
  ),
  info: (
    <svg className="w-5 h-5 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  warning: (
    <svg className="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>
  ),
};

// ═══════════════════════════════════════════════════════════════
// TOAST ITEM
// ═══════════════════════════════════════════════════════════════

interface ToastItemProps {
  toast: Toast;
  onDismiss: (id: string) => void;
}

const ToastItem: React.FC<ToastItemProps> = ({ toast, onDismiss }) => {
  useEffect(() => {
    const timer = setTimeout(() => {
      onDismiss(toast.id);
    }, toast.duration || 4000);

    return () => clearTimeout(timer);
  }, [toast.id, toast.duration, onDismiss]);

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 20, scale: 0.95 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 20, scale: 0.95 }}
      transition={{ type: 'spring', stiffness: 400, damping: 30 }}
      className="
        pointer-events-auto
        relative flex items-center gap-3
        px-4 py-3 min-w-[280px] max-w-[400px]
        bg-black/60 backdrop-blur-xl
        border border-white/10
        rounded-xl shadow-2xl
        overflow-hidden
      "
    >
      {/* Glow effect */}
      <div
        className={`
          absolute inset-0 opacity-20 blur-xl
          ${toast.type === 'success' ? 'bg-emerald-500' : ''}
          ${toast.type === 'error' ? 'bg-red-500' : ''}
          ${toast.type === 'info' ? 'bg-blue-500' : ''}
          ${toast.type === 'warning' ? 'bg-amber-500' : ''}
        `}
      />

      {/* Icon */}
      <div className="relative z-10 flex-shrink-0">
        {icons[toast.type]}
      </div>

      {/* Content */}
      <div className="relative z-10 flex-1 min-w-0">
        <p className="text-sm font-medium text-white truncate">{toast.title}</p>
        {toast.message && (
          <p className="text-xs text-white/60 truncate mt-0.5">{toast.message}</p>
        )}
      </div>

      {/* Action button */}
      {toast.action && (
        <button
          onClick={() => {
            toast.action?.onClick();
            onDismiss(toast.id);
          }}
          className="
            relative z-10 flex-shrink-0
            px-2 py-1 text-xs font-medium
            text-cyan-400 hover:text-cyan-300
            transition-colors
          "
        >
          {toast.action.label}
        </button>
      )}

      {/* Dismiss button */}
      <button
        onClick={() => onDismiss(toast.id)}
        className="
          relative z-10 flex-shrink-0
          p-1 text-white/40 hover:text-white/80
          transition-colors
        "
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </motion.div>
  );
};

// ═══════════════════════════════════════════════════════════════
// PROVIDER
// ═══════════════════════════════════════════════════════════════

export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((type: ToastType, title: string, message?: string) => {
    const id = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    setToasts((prev) => [...prev, { id, type, title, message }]);
    return id;
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const dismissAll = useCallback(() => {
    setToasts([]);
  }, []);

  const value: ToastContextValue = {
    toasts,
    success: (title, message) => addToast('success', title, message),
    error: (title, message) => addToast('error', title, message),
    info: (title, message) => addToast('info', title, message),
    warning: (title, message) => addToast('warning', title, message),
    dismiss,
    dismissAll,
  };

  return (
    <ToastContext.Provider value={value}>
      {children}

      {/* Toast Container */}
      <div
        className="
          fixed z-[9999] pointer-events-none
          flex flex-col gap-2
          
          /* Desktop: Bottom-right */
          bottom-4 right-4
          
          /* Mobile: Top-center */
          sm:bottom-4 sm:right-4 sm:top-auto sm:left-auto sm:items-end
          max-sm:top-4 max-sm:left-1/2 max-sm:-translate-x-1/2 max-sm:items-center max-sm:right-auto max-sm:bottom-auto
        "
      >
        <AnimatePresence mode="popLayout">
          {toasts.map((toast) => (
            <ToastItem key={toast.id} toast={toast} onDismiss={dismiss} />
          ))}
        </AnimatePresence>
      </div>
    </ToastContext.Provider>
  );
};

export default ToastProvider;
