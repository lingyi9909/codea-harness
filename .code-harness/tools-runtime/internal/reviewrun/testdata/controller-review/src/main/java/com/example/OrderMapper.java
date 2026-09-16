package com.example;

import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface OrderMapper {
    void insertOrder();
    void cancelOrder();
}
