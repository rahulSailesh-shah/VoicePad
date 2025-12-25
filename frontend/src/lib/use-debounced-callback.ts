import { useCallback, useRef, useEffect } from "react";

/**
 * A hook that returns a debounced version of the provided callback function.
 * The debounced function will only execute after the specified delay has passed
 * since the last time it was invoked. If called again before the delay expires,
 * the timer resets and the latest arguments are used.
 *
 * @param callback - The function to debounce
 * @param delay - The delay in milliseconds (default: 1000ms)
 * @returns A debounced version of the callback function
 *
 * @example
 * ```tsx
 * const debouncedSave = useDebouncedCallback((data) => {
 *   saveToDatabase(data);
 * }, 1000);
 *
 * // Call debouncedSave multiple times rapidly
 * debouncedSave(data1); // Timer starts
 * debouncedSave(data2); // Timer resets, data1 is discarded
 * debouncedSave(data3); // Timer resets, data2 is discarded
 * // After 1 second of no calls, saveToDatabase(data3) executes
 * ```
 */
export function useDebouncedCallback<T extends (...args: any[]) => void>(
  callback: T,
  delay: number = 1000
): T {
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingArgsRef = useRef<Parameters<T> | null>(null);
  const callbackRef = useRef(callback);

  // Keep callback ref up to date
  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  const debouncedCallback = useCallback(
    ((...args: Parameters<T>) => {
      // Store the latest arguments
      pendingArgsRef.current = args;

      // Clear existing timer
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }

      // Set new timer
      timeoutRef.current = setTimeout(() => {
        if (pendingArgsRef.current && callbackRef.current) {
          callbackRef.current(...pendingArgsRef.current);
          pendingArgsRef.current = null;
        }
      }, delay);
    }) as T,
    [delay]
  );

  return debouncedCallback;
}
