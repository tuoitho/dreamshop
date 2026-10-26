package com.ecommerce.identity.security.config;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.Socket;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.core.env.Environment;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class ManagementPortIsolationTest extends PostgresIntegrationTestSupport {
    @LocalServerPort
    int serverPort;

    @Autowired
    Environment environment;

    @Test
    void exposesActuatorOnlyOnManagementPortAndBlocksMainApplicationPort() throws Exception {
        Integer managementPort = environment.getProperty("local.management.port", Integer.class);
        assertThat(managementPort).isNotNull().isNotEqualTo(serverPort);

        HttpClient client = HttpClient.newHttpClient();

        HttpResponse<String> mainPortHealth = client.send(
                HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + serverPort + "/actuator/health"))
                        .GET()
                        .build(),
                HttpResponse.BodyHandlers.ofString());
        assertThat(mainPortHealth.statusCode()).isEqualTo(401);

        try (Socket socket = new Socket("127.0.0.1", serverPort);
                BufferedReader reader = new BufferedReader(
                        new InputStreamReader(socket.getInputStream(), StandardCharsets.US_ASCII))) {
            String rawRequest = "GET /actuator/health HTTP/1.1\r\n"
                    + "Host: 127.0.0.1:" + managementPort + "\r\n"
                    + "Connection: close\r\n\r\n";
            socket.getOutputStream().write(rawRequest.getBytes(StandardCharsets.US_ASCII));
            socket.getOutputStream().flush();
            assertThat(reader.readLine()).contains("401");
        }

        HttpResponse<String> mgmtHealth = client.send(
                HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + managementPort + "/actuator/health"))
                        .GET()
                        .build(),
                HttpResponse.BodyHandlers.ofString());
        assertThat(mgmtHealth.statusCode()).isEqualTo(200);
        assertThat(mgmtHealth.body()).contains("\"status\":\"UP\"");

        HttpResponse<String> mgmtReadiness = client.send(
                HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + managementPort + "/actuator/health/readiness"))
                        .GET()
                        .build(),
                HttpResponse.BodyHandlers.ofString());
        assertThat(mgmtReadiness.statusCode()).isEqualTo(200);
        assertThat(mgmtReadiness.body()).contains("\"status\":\"UP\"");
    }
}
