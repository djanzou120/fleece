'use client';
/**
 * Toast notification — lightweight inline component.
 * DEBT: Replace with shadcn/ui Toast (which uses @radix-ui/react-toast) once online.
 */
import React, { createContext, useCallback, useContext, useState } from 'react';

type ToastVariant = 'success' | 'error' | 'info';

interface ToastItem {
  id: string;
  message: string;
  variant: ToastVariant;
}

interface ToastContextValue {
  toast: (message: string, variant?: ToastVariant) => void;
}

const ToastContext = createContext<ToastContextValue>({ toast: () => {} });

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const toast = useCallback((message: string, variant: ToastVariant = 'info') => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [...prev, { id, message, variant }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4000);
  }, []);

  const variantColor: Record<ToastVariant, string> = {
    success: 'var(--accent-text)',
    error: 'var(--danger)',
    info: 'var(--info)',
  };

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      {/* Toast container */}
      <div
        aria-live="polite"
        aria-atomic="false"
        style={{
          position: 'fixed',
          bottom: '24px',
          right: '24px',
          zIndex: 200,
          display: 'flex',
          flexDirection: 'column',
          gap: '8px',
          pointerEvents: 'none',
        }}
      >
        {toasts.map((t) => (
          <div
            key={t.id}
            role="status"
            style={{
              backgroundColor: 'var(--surface)',
              border: '1px solid var(--border)',
              borderLeft: `3px solid ${variantColor[t.variant]}`,
              borderRadius: '9px',
              padding: '12px 16px',
              fontSize: '13px',
              color: 'var(--text)',
              boxShadow: '0 4px 16px rgba(0,0,0,0.4)',
              animation: 'flc-toast 0.3s ease both',
              maxWidth: '340px',
              pointerEvents: 'auto',
            }}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  return useContext(ToastContext);
}
