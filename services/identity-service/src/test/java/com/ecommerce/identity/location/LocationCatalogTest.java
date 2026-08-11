package com.ecommerce.identity.location;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;
import tools.jackson.databind.ObjectMapper;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class LocationCatalogTest {
    private final LocationCatalog catalog = catalog();

    private static LocationCatalog catalog() {
        try {
            return new LocationCatalog(new ObjectMapper(),
                    new ClassPathResource("locations/vietnam-administrative-units.json"));
        } catch (IOException exception) {
            throw new UncheckedIOException(exception);
        }
    }

    @Test
    void loadsSnapshotWithThirtyFourUniqueProvinces() {
        assertThat(catalog.provinces()).hasSize(34);
        assertThat(catalog.provinces().stream().map(Province::code).distinct()).hasSize(34);
    }

    @Test
    void hasUniqueWardCodesWithinEachProvince() {
        for (Province province : catalog.provinces()) {
            List<String> wardCodes = province.wards().stream().map(Ward::code).toList();
            assertThat(wardCodes).as("province %s", province.code()).doesNotHaveDuplicates();
        }
    }

    @Test
    void knowsWardBelongsToProvince() {
        assertThat(catalog.wardBelongsToProvince("79", "26740")).isTrue();
    }

    @Test
    void rejectsWardFromAnotherProvince() {
        assertThat(catalog.wardBelongsToProvince("79", "00043")).isFalse();
    }

    @Test
    void exposesProvinceNamesForApi() {
        assertThat(catalog.province("79").orElseThrow().name()).isEqualTo("Thành phố Hồ Chí Minh");
        assertThat(catalog.province("01").orElseThrow().name()).isEqualTo("Thành phố Hà Nội");
    }

    @Test
    void rejectsUnknownProvince() {
        assertThat(catalog.province("999")).isEmpty();
        assertThat(catalog.wardBelongsToProvince("999", "26740")).isFalse();
    }

    @Test
    void exposesImmutableSnapshot() {
        assertThatThrownBy(() -> catalog.provinces().clear())
                .isInstanceOf(UnsupportedOperationException.class);
        assertThatThrownBy(() -> catalog.province("79").orElseThrow().wards().clear())
                .isInstanceOf(UnsupportedOperationException.class);
    }
}
