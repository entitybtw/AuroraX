export declare function waitForReason<T>(value: T | PromiseLike<T>, signal: AbortSignal, onLate?: (value: T) => void): Promise<T>;
