/**
 * Utility helpers for the Fleece dashboard.
 */

/**
 * Format a wallet balance from smallest unit (centimes) to display string.
 * e.g. formatBalance(150000, 'XOF') => '1 500,00 XOF'
 */
export function formatBalance(amountCentimes: number, currency: string): string {
  const amount = amountCentimes / 100;
  try {
    return new Intl.NumberFormat('fr-FR', {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
    }).format(amount);
  } catch {
    // Fallback for unknown currency codes
    return `${amount.toFixed(2)} ${currency}`;
  }
}

/**
 * Format an ISO date string to a readable local date.
 */
export function formatDate(isoString: string): string {
  try {
    return new Intl.DateTimeFormat('fr-FR', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
    }).format(new Date(isoString));
  } catch {
    return isoString;
  }
}

/**
 * Format an ISO date string to a readable local date with time.
 */
export function formatDateTime(isoString: string): string {
  try {
    return new Intl.DateTimeFormat('fr-FR', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(isoString));
  } catch {
    return isoString;
  }
}

/**
 * Truncate a string to a max length, appending '...' if needed.
 */
export function truncate(str: string, maxLength: number): string {
  if (str.length <= maxLength) return str;
  return str.slice(0, maxLength) + '...';
}

/**
 * Combine class names, filtering out falsy values.
 * Minimal replacement for clsx/cn — no external dependency.
 */
export function cn(...classes: (string | undefined | null | false)[]): string {
  return classes.filter(Boolean).join(' ');
}
