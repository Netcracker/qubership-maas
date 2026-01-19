package org.qubership.it.maas;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.*;

import java.io.IOException;
import java.util.Base64;

public class K8sAuthHelper {
    private final OkHttpClient httpClient;
    private final String issuer;

    public K8sAuthHelper(String issuer) {
        this.httpClient = new OkHttpClient();
        this.issuer = issuer;
    }

    public String getServiceAccountToken() {
        String headerJson = "{\"alg\":\"none\",\"typ\":\"JWT\"}";
        String encodedHeader = base64UrlEncode(headerJson);

        String payloadJson = String.format("{\"iss\":\"%s\"}", issuer);
        String encodedPayload = base64UrlEncode(payloadJson);

        String signature = "";
        return String.format("%s.%s.%s", encodedHeader, encodedPayload, signature);
    }

    public String getMaasToken() throws IOException {
        RequestBody formBody = new FormBody.Builder()
                .add("grant_type", "client_credentials")
                .add("client_id", "test-client")
                .add("client_secret", "test-secret")
                .add("scope", "openid")
                .build();
        Request request = new Request.Builder()
                .url(issuer+"/token")
                .post(formBody)
                .build();
        try (Response response = httpClient.newCall(request).execute()) {
            String body = response.body().string();
            ObjectMapper mapper = new ObjectMapper();
            JsonNode json = mapper.readTree(body);
            return json.get("access_token").asText();
        }
    }

    private String base64UrlEncode(String data) {
        return Base64.getUrlEncoder()
                .withoutPadding()
                .encodeToString(data.getBytes());
    }
}
