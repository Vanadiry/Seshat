const locks = new Set();

export function registerLock(reset) {
    locks.add(reset);
    return function () {
        locks.delete(reset);
    };
}

export function resetOtherLocks(current) {
    for (const reset of locks) {
        if (reset !== current) reset();
    }
}
