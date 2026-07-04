create table admins
(
    id            bigint unsigned auto_increment
        primary key,
    username      varchar(64)                  not null,
    email         varchar(128)                 not null,
    name          varchar(64)                  null,
    password_hash varchar(255)                 not null,
    status        varchar(16) default 'active' not null,
    last_login_at datetime(3)                  null,
    created_at    datetime(3)                  null,
    updated_at    datetime(3)                  null,
    deleted_at    datetime(3)                  null,
    constraint idx_admins_email
        unique (email),
    constraint idx_admins_username
        unique (username)
);

create index idx_admins_deleted_at
    on admins (deleted_at);

create index idx_admins_status
    on admins (status);

create table app_states
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    `key`      varchar(255)                              not null,
    value      text                                      null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_app_states_unique_active
        unique (app_name, `key`, deleted_at)
)
    collate = utf8mb4_unicode_ci;

create index idx_app_states_expires
    on app_states (expires_at);

create table mind_maps
(
    id         bigint unsigned auto_increment
        primary key,
    paper_id   varchar(36) not null,
    owner_id   varchar(64) not null,
    graph_json longtext    null,
    created_at datetime(3) null,
    updated_at datetime(3) null,
    constraint idx_owner_paper_mind_map
        unique (paper_id, owner_id)
);

create index idx_mind_maps_owner_id
    on mind_maps (owner_id);

create table paper_annotations
(
    id            bigint unsigned auto_increment
        primary key,
    paper_id      varchar(36) not null,
    owner_id      varchar(64) not null,
    page_no       bigint      not null,
    text          text        null,
    note          text        null,
    translation   text        null,
    color         varchar(32) not null,
    bounding_rect text        null,
    rects         text        null,
    created_at    datetime(3) null,
    updated_at    datetime(3) null
);

create index idx_paper_annotations_owner_id
    on paper_annotations (owner_id);

create index idx_paper_annotations_page_no
    on paper_annotations (page_no);

create index idx_paper_annotations_paper_id
    on paper_annotations (paper_id);

create table paper_metas
(
    paper_id           varchar(36)  not null
        primary key,
    authors            text         null,
    affiliations       text         null,
    publish_year       bigint       null,
    venue              varchar(256) null,
    abstract           text         null,
    keywords           text         null,
    research_questions text         null,
    methods            text         null,
    experiments        text         null,
    results            text         null,
    innovations        text         null,
    limitations        text         null,
    future_work        text         null,
    updated_at         datetime(3)  null
);

create table paper_reports
(
    id          bigint unsigned auto_increment
        primary key,
    paper_id    varchar(36) not null,
    report_type varchar(16) not null,
    content     longtext    null,
    meta        text        null,
    created_at  datetime(3) null,
    updated_at  datetime(3) null,
    constraint idx_paper_report_type
        unique (paper_id, report_type)
);

create table paper_sections
(
    id        bigint unsigned auto_increment
        primary key,
    paper_id  varchar(36)  not null,
    level     bigint       null,
    title     varchar(512) null,
    page_no   bigint       null,
    order_idx bigint       null,
    x1        double       null,
    y1        double       null,
    x2        double       null,
    y2        double       null
);

create index idx_paper_sections_order_idx
    on paper_sections (order_idx);

create index idx_paper_sections_paper_id
    on paper_sections (paper_id);

create table paper_semantic_relations
(
    id                           bigint unsigned auto_increment
        primary key,
    owner_id                     varchar(64)                            not null,
    source_paper_id              varchar(36)                            not null,
    target_paper_id              varchar(36)                            not null,
    relation_type                varchar(64) default 'SEMANTIC_SIMILAR' not null,
    score                        double                                 null,
    matched_fields               text                                   null,
    summary                      text                                   null,
    keyword_similarity           text                                   null,
    research_question_similarity text                                   null,
    method_similarity            text                                   null,
    experiment_similarity        text                                   null,
    innovation_similarity        text                                   null,
    model                        varchar(128)                           null,
    created_at                   datetime(3)                            null,
    updated_at                   datetime(3)                            null,
    constraint idx_owner_semantic_pair
        unique (owner_id, source_paper_id, target_paper_id)
);

create index idx_paper_semantic_relations_owner_id
    on paper_semantic_relations (owner_id);

create index idx_paper_semantic_relations_source_paper_id
    on paper_semantic_relations (source_paper_id);

create index idx_paper_semantic_relations_target_paper_id
    on paper_semantic_relations (target_paper_id);

create table paper_tags
(
    paper_id varchar(36)     not null,
    tag_id   bigint unsigned not null,
    primary key (paper_id, tag_id)
);

create table papers
(
    id             varchar(36)   not null
        primary key,
    owner_id       varchar(64)   not null,
    title          varchar(512)  null,
    file_name      varchar(512)  not null,
    file_uri       varchar(1024) not null,
    size           bigint        null,
    status         varchar(16)   not null,
    fail_reason    varchar(512)  null,
    page_count     bigint        null,
    category       varchar(64)   null,
    parse_progress bigint        null,
    parsed_pages   bigint        null,
    total_pages    bigint        null,
    progress       bigint        null,
    last_read_page bigint        null,
    created_at     datetime(3)   null,
    updated_at     datetime(3)   null,
    deleted_at     datetime(3)   null
);

create index idx_papers_category
    on papers (category);

create index idx_papers_deleted_at
    on papers (deleted_at);

create index idx_papers_owner_id
    on papers (owner_id);

create index idx_papers_status
    on papers (status);

create table service_call_logs
(
    id            bigint unsigned auto_increment
        primary key,
    service_type  varchar(32)   not null,
    actor_id      varchar(64)   null,
    paper_id      varchar(36)   null,
    session_id    varchar(36)   null,
    success       tinyint(1)    null,
    duration_ms   bigint        null,
    error_message varchar(1024) null,
    created_at    datetime(3)   null
);

create index idx_service_call_logs_actor_id
    on service_call_logs (actor_id);

create index idx_service_call_logs_created_at
    on service_call_logs (created_at);

create index idx_service_call_logs_paper_id
    on service_call_logs (paper_id);

create index idx_service_call_logs_service_type
    on service_call_logs (service_type);

create index idx_service_call_logs_session_id
    on service_call_logs (session_id);

create index idx_service_call_logs_success
    on service_call_logs (success);

create table session_events
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    event      json                                      not null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null
)
    collate = utf8mb4_unicode_ci;

create index idx_session_events_expires
    on session_events (expires_at);

create index idx_session_events_lookup
    on session_events (app_name, user_id, session_id, created_at);

create table session_states
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    state      json                                      null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_session_states_unique_active
        unique (app_name, user_id, session_id, deleted_at)
)
    collate = utf8mb4_unicode_ci;

create index idx_session_states_expires
    on session_states (expires_at);

create table session_summaries
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    filter_key varchar(255) default ''                   not null,
    summary    json                                      null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_session_summaries_unique_active
        unique (app_name(191), user_id(191), session_id(191), filter_key(191))
)
    collate = utf8mb4_unicode_ci;

create index idx_session_summaries_expires
    on session_summaries (expires_at);

create table session_track_events
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    track      varchar(255)                              not null,
    event      json                                      not null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null
)
    collate = utf8mb4_unicode_ci;

create index idx_session_track_events_expires
    on session_track_events (expires_at);

create index idx_session_track_events_lookup
    on session_track_events (app_name, user_id, session_id, created_at);

create table sessions
(
    id         varchar(36)  not null
        primary key,
    student_id varchar(64)  not null,
    paper_id   varchar(36)  null,
    agent_type varchar(16)  null,
    topic_id   varchar(36)  null,
    topic_vec  longtext     null,
    title      varchar(255) null,
    created_at datetime(3)  null,
    updated_at datetime(3)  null,
    deleted_at datetime(3)  null
);

create index idx_sessions_agent_type
    on sessions (agent_type);

create index idx_sessions_deleted_at
    on sessions (deleted_at);

create index idx_sessions_paper_id
    on sessions (paper_id);

create index idx_sessions_student_id
    on sessions (student_id);

create index idx_sessions_topic_id
    on sessions (topic_id);

create table tags
(
    id       bigint unsigned auto_increment
        primary key,
    owner_id varchar(64) not null,
    name     varchar(64) not null
);

create index idx_owner_name
    on tags (owner_id, name);

create table test_history_app_states
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    `key`      varchar(255)                              not null,
    value      text                                      null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_test_history_app_states_unique_active
        unique (app_name, `key`, deleted_at)
)
    collate = utf8mb4_unicode_ci;

create index idx_test_history_app_states_expires
    on test_history_app_states (expires_at);

create table test_history_session_events
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    event      json                                      not null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null
)
    collate = utf8mb4_unicode_ci;

create index idx_test_history_session_events_expires
    on test_history_session_events (expires_at);

create index idx_test_history_session_events_lookup
    on test_history_session_events (app_name, user_id, session_id, created_at);

create table test_history_session_states
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    state      json                                      null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_test_history_session_states_unique_active
        unique (app_name, user_id, session_id, deleted_at)
)
    collate = utf8mb4_unicode_ci;

create index idx_test_history_session_states_expires
    on test_history_session_states (expires_at);

create table test_history_session_summaries
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    filter_key varchar(255) default ''                   not null,
    summary    json                                      null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_test_history_session_summaries_unique_active
        unique (app_name(191), user_id(191), session_id(191), filter_key(191))
)
    collate = utf8mb4_unicode_ci;

create index idx_test_history_session_summaries_expires
    on test_history_session_summaries (expires_at);

create table test_history_session_track_events
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    session_id varchar(255)                              not null,
    track      varchar(255)                              not null,
    event      json                                      not null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null
)
    collate = utf8mb4_unicode_ci;

create index idx_test_history_session_track_events_expires
    on test_history_session_track_events (expires_at);

create index idx_test_history_session_track_events_lookup
    on test_history_session_track_events (app_name, user_id, session_id, created_at);

create table test_history_user_states
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    `key`      varchar(255)                              not null,
    value      text                                      null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_test_history_user_states_unique_active
        unique (app_name, user_id, `key`, deleted_at)
)
    collate = utf8mb4_unicode_ci;

create index idx_test_history_user_states_expires
    on test_history_user_states (expires_at);

create table topics
(
    id           varchar(36)      not null
        primary key,
    student_id   varchar(64)      not null,
    agent_type   varchar(16)      null,
    name         varchar(64)      null,
    centroid     longtext         null,
    member_count bigint default 0 not null,
    created_at   datetime(3)      null,
    updated_at   datetime(3)      null,
    deleted_at   datetime(3)      null
);

create index idx_topics_agent_type
    on topics (agent_type);

create index idx_topics_deleted_at
    on topics (deleted_at);

create index idx_topics_student_id
    on topics (student_id);

create table user_states
(
    id         bigint auto_increment
        primary key,
    app_name   varchar(255)                              not null,
    user_id    varchar(255)                              not null,
    `key`      varchar(255)                              not null,
    value      text                                      null,
    created_at timestamp(6) default CURRENT_TIMESTAMP(6) not null,
    updated_at timestamp(6) default CURRENT_TIMESTAMP(6) not null on update CURRENT_TIMESTAMP(6),
    expires_at timestamp(6)                              null,
    deleted_at timestamp(6)                              null,
    constraint idx_user_states_unique_active
        unique (app_name, user_id, `key`, deleted_at)
)
    collate = utf8mb4_unicode_ci;

create index idx_user_states_expires
    on user_states (expires_at);

create table users
(
    id            bigint unsigned auto_increment
        primary key,
    student_id    varchar(64)  not null,
    name          varchar(64)  null,
    email         varchar(128) null,
    avatar_url    varchar(255) null,
    class_id      varchar(64)  null,
    password_hash varchar(255) not null,
    created_at    datetime(3)  null,
    updated_at    datetime(3)  null,
    deleted_at    datetime(3)  null,
    constraint idx_users_student_id
        unique (student_id)
);

create index idx_users_class_id
    on users (class_id);

create index idx_users_deleted_at
    on users (deleted_at);

create index idx_users_email
    on users (email);


