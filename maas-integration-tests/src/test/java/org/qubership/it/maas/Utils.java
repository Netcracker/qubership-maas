package org.qubership.it.maas;

import lombok.extern.slf4j.Slf4j;

import java.util.Base64;

@Slf4j
public class Utils {
    @FunctionalInterface
    interface OmnivoreRunnable {
        void run() throws Exception;
    }

    public static void retry(int maxAttempts, OmnivoreRunnable f) throws Exception {
        var attempt = 0;
        while (true) {
            try {
                f.run();
                break;
            } catch (Throwable e) {
                if (attempt++ < maxAttempts) {
                    log.error("test error: {}, attempt {} of {}. Retry after {}sec", e.getMessage(), attempt, maxAttempts, attempt);
                    Thread.sleep(1000 * attempt);
                } else {
                    throw e;
                }
            }
        }
    }
}
