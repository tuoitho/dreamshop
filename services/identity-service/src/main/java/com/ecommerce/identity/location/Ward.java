package com.ecommerce.identity.location;

/**
 * One ward/commune from the vendored administrative snapshot.
 *
 * @param code zero-padded administrative code, for example "26740"
 * @param name display name, for example "Phường Bến Nghé"
 */
public record Ward(String code, String name) {
}
