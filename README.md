# Гофермарт - накопительная система лояльности

HTTP API накопительной системы лояльности интернет-магазина "Гофермарт":
регистрация покупателей, приём номеров заказов, начисление баллов через внешнюю
систему расчёта и списание баллов в счёт новых заказов.


| Возможность | Реализация |
|---|---|
| Регистрация, аутентификация и авторизация | `POST /api/user/register`, `POST /api/user/login`; пароли — bcrypt; сессия — JWT в заголовке `Authorization: Bearer` и в cookie `token`; после регистрации пользователь сразу аутентифицирован |
| Приём номеров заказов | `POST /api/user/orders`, тело `text/plain`; номер проверяется алгоритмом Луна |
| Список номеров заказов пользователя | `GET /api/user/orders`, сортировка от новых к старым, даты в RFC3339, статусы `NEW/PROCESSING/INVALID/PROCESSED` |
| Накопительный счёт | `GET /api/user/balance` — текущий баланс и сумма списаний за всё время |
| Проверка заказов во внешней системе расчёта | фоновый воркер опрашивает `GET {ACCRUAL_SYSTEM_ADDRESS}/api/orders/{number}`, обрабатывает `200/204/429 (Retry-After)/500` |
| Начисление вознаграждения | при статусе `PROCESSED` баллы зачисляются на счёт владельца в одной транзакции с обновлением заказа |
| Списание баллов | `POST /api/user/balance/withdraw` — списание под гипотетический номер заказа, проверка достаточности средств (`402`) и номера по Луну (`422`) |
| Информация о списаниях | `GET /api/user/withdrawals`, сортировка от новых к старым, даты в RFC3339 |
| Хранилище | PostgreSQL, миграции goose встроены в бинарь и применяются на старте |
| Сжатие | приём `Content-Encoding: gzip` и ответ `gzip` при `Accept-Encoding: gzip` |

## Конфигурация

Переменные окружения имеют приоритет над флагами.

| Параметр | Флаг | Переменная окружения | По умолчанию |
|---|---|---|---|
| Адрес и порт сервиса | `-a` | `RUN_ADDRESS` | `:8080` |
| Подключение к БД | `-d` | `DATABASE_URI` | — |
| Адрес системы расчёта | `-r` | `ACCRUAL_SYSTEM_ADDRESS` | — |
| Секрет подписи JWT | — | `JWT_SECRET` | `gophermart-dev-secret` |
| Уровень логирования | — | `LOG_LEVEL` | `info` |

## Запуск

```sh
# система расчёта начислений
cmd/accrual/accrual_linux_amd64 -a :8081

# сборка и запуск сервиса
go build -o gophermart ./cmd/gophermart
./gophermart \
  -a :8080 \
  -d "postgresql://postgres:postgres@localhost:5432/praktikum?sslmode=disable" \
  -r http://localhost:8081
```

## Тесты

```sh
go test ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
go vet ./...
```

## Проверка через curl

Базовый адрес: `http://localhost:8080`. JWT возвращается в заголовке ответа
`Authorization` на `register` / `login` — сохраните его и передавайте в запросах.

```sh
BASE=http://localhost:8080

# 1. Регистрация (200; токен в заголовке Authorization ответа)
TOKEN=$(curl -si -X POST "$BASE/api/user/register" \
  -H 'Content-Type: application/json' \
  -d '{"login":"alice","password":"secret"}' \
  | grep -i '^Authorization:' | tr -d '\r' | awk '{print $3}')
echo "$TOKEN"

# Повторная регистрация того же логина -> 409
curl -si -X POST "$BASE/api/user/register" \
  -H 'Content-Type: application/json' \
  -d '{"login":"alice","password":"secret"}' | head -n 1

# 2. Аутентификация (200 / 401)
curl -si -X POST "$BASE/api/user/login" \
  -H 'Content-Type: application/json' \
  -d '{"login":"alice","password":"secret"}' | head -n 1

# 3. Загрузка номера заказа: 202 новый, 200 повторный от того же пользователя,
#    409 чужой, 422 не проходит Луна
curl -si -X POST "$BASE/api/user/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  -d '12345678903' | head -n 1

curl -si -X POST "$BASE/api/user/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  -d '12345678901' | head -n 1        # 422

# 4. Список заказов пользователя (200 с телом / 204)
curl -s "$BASE/api/user/orders" -H "Authorization: Bearer $TOKEN"

# 5. Баланс (200)
curl -s "$BASE/api/user/balance" -H "Authorization: Bearer $TOKEN"

# 6. Списание баллов (200 / 402 недостаточно средств / 422 неверный номер)
curl -si -X POST "$BASE/api/user/balance/withdraw" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"order":"2377225624","sum":100}' | head -n 1

# 7. История списаний (200 с телом / 204)
curl -s "$BASE/api/user/withdrawals" -H "Authorization: Bearer $TOKEN"

# Любой /api/user/* без токена -> 401
curl -si "$BASE/api/user/balance" | head -n 1

# Сжатие: gzip-запрос и gzip-ответ
curl -s "$BASE/api/user/orders" -H "Authorization: Bearer $TOKEN" \
  -H 'Accept-Encoding: gzip' --compressed
```

## Обновление шаблона автотестов

```sh
git remote add -m master template https://github.com/yandex-praktikum/go-musthave-diploma-tpl.git
git fetch template && git checkout template/master .github
```
