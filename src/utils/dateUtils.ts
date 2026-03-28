/**
 * Date utility functions for categorizing files by modification date
 */

export function categorizeDate(timestamp: number): string {
  const now = Date.now();
  const diff = now - timestamp;
  const hours = diff / (1000 * 60 * 60);
  const days = hours / 24;
  const years = days / 365;

  if (hours < 1) {
    return 'Last Hour';
  } else if (hours < 24) {
    return 'Today';
  } else if (days < 7) {
    return 'This Week';
  } else if (days < 30) {
    return 'This Month';
  } else if (days < 90) {
    return 'Last 3 Months';
  } else if (days < 180) {
    return 'Last 6 Months';
  } else if (days < 365) {
    return 'This Year';
  } else if (years < 2) {
    return '1-2 Years Ago';
  } else if (years < 5) {
    return '2-5 Years Ago';
  } else {
    return '5+ Years Ago';
  }
}

export function formatDate(timestamp: number): string {
  const date = new Date(timestamp);
  return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
}


