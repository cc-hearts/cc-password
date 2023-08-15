create table password_category
(
    id       int auto_increment
        primary key,
    category varchar(255) collate utf8mb4_general_ci null,
    uid      int                                     null,
    constraint unique_uid_category
        unique (uid, category)
);

