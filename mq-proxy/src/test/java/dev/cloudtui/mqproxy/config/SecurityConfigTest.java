package dev.cloudtui.mqproxy.config;

import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.httpBasic;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

/**
 * Confirms every endpoint requires HTTP Basic Auth: missing or wrong
 * credentials get 401; valid credentials (from the "local" profile's
 * fixed dev user) get past the security filter and reach the real
 * {@code GET /queues} handler (200).
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("local")
class SecurityConfigTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void rejectsRequestsWithNoCredentials() throws Exception {
        mockMvc.perform(get("/queues"))
            .andExpect(status().isUnauthorized());
    }

    @Test
    void rejectsRequestsWithBadCredentials() throws Exception {
        mockMvc.perform(get("/queues").with(httpBasic("admin", "wrong-password")))
            .andExpect(status().isUnauthorized());
    }

    @Test
    void acceptsRequestsWithValidCredentials() throws Exception {
        mockMvc.perform(get("/queues").with(httpBasic("admin", "admin")))
            .andExpect(status().isOk());
    }
}
