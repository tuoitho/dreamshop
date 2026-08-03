package com.ecommerce.identity.user.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;

@Entity
@Table(name = "roles")
public class RoleEntity {
    @Id
    private Short id;

    @Column(nullable = false, length = 32)
    private String code;

    protected RoleEntity() {
    }

    String code() {
        return code;
    }
}
