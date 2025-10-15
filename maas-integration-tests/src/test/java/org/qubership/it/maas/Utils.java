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

    public static String getNewJwt(String issuer) {
        String headerJson = "{\"alg\":\"none\",\"typ\":\"JWT\"}";
        String encodedHeader = base64UrlEncode(headerJson);

        String payloadJson = String.format("{\"iss\":\"%s\"}", issuer);
        String encodedPayload = base64UrlEncode(payloadJson);

        String signature = "";
        return String.format("%s.%s.%s", encodedHeader, encodedPayload, signature);
    }

    private static String base64UrlEncode(String data) {
        return Base64.getUrlEncoder()
                .withoutPadding()
                .encodeToString(data.getBytes());
    }
}
