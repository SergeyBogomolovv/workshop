-- Атомарное создание схемы и каталога с последовательным выполнением параллельных миграций.
BEGIN;

SELECT pg_advisory_xact_lock(746219);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version integer PRIMARY KEY
);

DO $migration$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 1) THEN
        CREATE TABLE users (
            id uuid PRIMARY KEY,
            login_cipher text NOT NULL,
            login_index text NOT NULL UNIQUE,
            password_hash text NOT NULL,
            name_cipher text NOT NULL,
            phone_cipher text NOT NULL,
            email_cipher text NOT NULL,
            role text NOT NULL DEFAULT 'client'
                CHECK (role IN ('client', 'admin')),
            policy_version text NOT NULL,
            consent_at timestamptz NOT NULL DEFAULT now(),
            created_at timestamptz NOT NULL DEFAULT now()
        );

        CREATE TABLE sessions (
            id uuid PRIMARY KEY,
            user_id uuid NOT NULL REFERENCES users(id),
            token_hash text NOT NULL UNIQUE,
            csrf_token text NOT NULL,
            created_at timestamptz NOT NULL DEFAULT now(),
            expires_at timestamptz NOT NULL,
            revoked boolean NOT NULL DEFAULT false
        );

        CREATE INDEX sessions_user_idx ON sessions(user_id);

        CREATE TABLE device_categories (
            code text PRIMARY KEY,
            name text NOT NULL
        );

        CREATE TABLE services (
            id uuid PRIMARY KEY,
            name text NOT NULL,
            description text NOT NULL,
            price_kopecks integer NOT NULL CHECK (price_kopecks >= 0),
            active boolean NOT NULL DEFAULT true
        );

        CREATE TABLE service_categories (
            service_id uuid NOT NULL REFERENCES services(id),
            category_code text NOT NULL REFERENCES device_categories(code),
            PRIMARY KEY (service_id, category_code)
        );

        CREATE TABLE appointments (
            id uuid PRIMARY KEY,
            user_id uuid NOT NULL REFERENCES users(id),
            service_id uuid NOT NULL,
            category_code text NOT NULL,
            slot_start timestamptz NOT NULL,
            description_cipher text NOT NULL,
            status text NOT NULL CHECK (
                status IN (
                    'new',
                    'accepted',
                    'in_progress',
                    'ready',
                    'completed',
                    'cancelled'
                )
            ),
            created_at timestamptz NOT NULL DEFAULT now(),
            updated_at timestamptz NOT NULL DEFAULT now(),
            FOREIGN KEY (service_id, category_code)
                REFERENCES service_categories(service_id, category_code)
        );

        CREATE UNIQUE INDEX appointments_slot_unique
            ON appointments(slot_start)
            WHERE status <> 'cancelled';

        CREATE INDEX appointments_owner_idx
            ON appointments(user_id, created_at DESC);

        CREATE TABLE appointment_status_history (
            id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
            appointment_id uuid NOT NULL REFERENCES appointments(id),
            previous_status text NOT NULL,
            status text NOT NULL,
            actor_id uuid NOT NULL REFERENCES users(id),
            created_at timestamptz NOT NULL DEFAULT now()
        );

        CREATE INDEX history_appointment_idx
            ON appointment_status_history(appointment_id, id);

        INSERT INTO device_categories VALUES
            ('smartphone', 'Смартфон'),
            ('laptop', 'Ноутбук'),
            ('tablet', 'Планшет');

        INSERT INTO services (id, name, description, price_kopecks) VALUES
            (
                '10000000-0000-4000-8000-000000000001',
                'Диагностика',
                'Найдём причину неисправности и предложим решение.',
                50000
            ),
            (
                '10000000-0000-4000-8000-000000000002',
                'Замена дисплея',
                'Вернём чёткое изображение и отзывчивость экрана.',
                250000
            ),
            (
                '10000000-0000-4000-8000-000000000003',
                'Замена аккумулятора',
                'Поможем устройству снова держать заряд.',
                180000
            ),
            (
                '10000000-0000-4000-8000-000000000004',
                'Чистка и профилактика',
                'Очистим систему охлаждения и заменим термопасту.',
                200000
            ),
            (
                '10000000-0000-4000-8000-000000000005',
                'Настройка системы',
                'Устраним программные сбои и настроим устройство.',
                120000
            ),
            (
                '10000000-0000-4000-8000-000000000006',
                'Ремонт разъёма',
                'Восстановим надёжное подключение и зарядку.',
                150000
            );

        INSERT INTO service_categories
        SELECT s.id, c.code
        FROM services s
        CROSS JOIN device_categories c
        WHERE s.id <> '10000000-0000-4000-8000-000000000004'
            OR c.code = 'laptop';

        INSERT INTO schema_migrations VALUES (1);
    END IF;
END;
$migration$;

-- Приложение может только читать каталог и добавлять записи в историю.
REVOKE ALL ON schema_migrations FROM "user";

REVOKE INSERT, UPDATE, DELETE
    ON services, device_categories, service_categories
    FROM "user";

REVOKE UPDATE, DELETE ON appointment_status_history FROM "user";

COMMIT;
