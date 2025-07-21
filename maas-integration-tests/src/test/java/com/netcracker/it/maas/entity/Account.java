package com.netcracker.it.maas.entity;

import lombok.Builder;
import lombok.Data;

import java.util.List;

@Data
@Builder
public class Account {

    private String username;
    private String password;
    private List<String> roles;
    private String namespace;

    public Account() {
    }
    public Account(String username, String password, List<String> roles, String namespace) {
        this.username = username;
        this.password = password;
        this.roles = roles;
        this.namespace  = namespace;
    }

    public Account(String username, String password, List<String> roles) {
        this.username = username;
        this.password = password;
        this.roles = roles;
    }


}
