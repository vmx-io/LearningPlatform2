# SAP Quiz API

A backend service for running SAP Commerce certification practice exams and learning sessions.  
Built with **Go**, **Gin**, and **SQLite**.

---

## Features

- **Learning mode** — fetch questions, answer them, and get immediate feedback with explanations (EN + PL) and source URLs.
- **Exam mode** — start a timed exam (default: 80 questions, 3 hours), answer without feedback, see results at the end.
- **Anonymous user accounts** — created automatically via cookies, can set display name, export/restore account.
- **Statistics** — track number of questions answered, accuracy, and exam results (pass/fail).
- **Persistent storage** — all data stored in a local SQLite file (`quiz.db`).

---

## Data Model

Questions, options, and explanations are loaded from `data/questions.json`.  
Each explanation can include a `url` field for source references.

### Question JSON structure

```json
{
  "id": "001",
  "questionText": "Question text in English…",
  "options": [
    { "id": "a", "text": "Option A in English…" },
    { "id": "b", "text": "Option B in English…" },
    { "id": "c", "text": "Option C in English…" },
    { "id": "d", "text": "Option D in English…" }
  ],
  "optionsExplanation": {
    "en": [
      { "id": "a", "text": "English explanation for A…", "url": "https://example.com" },
      { "id": "b", "text": "English explanation for B…", "url": "" },
      { "id": "c", "text": "English explanation for C…", "url": "" },
      { "id": "d", "text": "English explanation for D…", "url": "" }
    ],
    "pl": [
      { "id": "a", "text": "Polish explanation for A…", "url": "https://example.com" },
      { "id": "b", "text": "Polish explanation for B…", "url": "" },
      { "id": "c", "text": "Polish explanation for C…", "url": "" },
      { "id": "d", "text": "Polish explanation for D…", "url": "" }
    ]
  },
  "multiSelect": true,
  "correctOptionIds": ["b", "c"]
}
```

---

## API Endpoints

### Questions & Learning

| Method | Endpoint                | Description |
|--------|------------------------|-------------|
| `GET`  | `/api/v1/questions`    | Get all questions (learning mode). Supports pagination. |
| `POST` | `/api/v1/learn/answer` | Submit an answer in learning mode and receive immediate correctness and explanations. |

**Example request:**

```bash
curl -b cookies.txt -X POST http://localhost:8080/api/v1/learn/answer   -H "Content-Type: application/json"   -d '{"questionId":"001","selected":["a","d"],"lang":"en"}'
```

---

### Exams

| Method | Endpoint                         | Description |
|--------|---------------------------------|-------------|
| `POST` | `/api/v1/exams`                 | Start a new exam (80 random questions, 3h). |
| `POST` | `/api/v1/exams/:id/answer`      | Submit an answer during an exam (no feedback). |
| `POST` | `/api/v1/exams/:id/finish`      | Finish an exam and get score + report. |
| `GET`  | `/api/v1/exams`                 | List user’s past exams. |
| `GET`  | `/api/v1/exams/:id`             | Retrieve details of a specific exam. |

---

### User

| Method | Endpoint                  | Description |
|--------|--------------------------|-------------|
| `GET`  | `/api/v1/me`             | Get current user profile. |
| `PUT`  | `/api/v1/me`             | Update current user profile (e.g. `displayName`). |
| `GET`  | `/api/v1/me/export-key`  | Export account restore key. |
| `POST` | `/api/v1/me/restore`     | Restore account using export key. |

---

### Stats

| Method | Endpoint         | Description |
|--------|-----------------|-------------|
| `GET`  | `/api/v1/stats` | Returns aggregated statistics for the current user (answered questions, accuracy, passed exams, failed exams, etc.). |

---

## Running the Server

```bash
go run .
```

By default, the server runs on **port 8080**.  
You can change the port with:

```bash
PORT=9090 go run .
```

---

## Seeding Questions

Questions are loaded automatically from `data/questions.json` if the `questions` table is empty.  
To re-import after changes, clear the database:

```bash
rm quiz.db
go run .
```

---

## Pass/Fail Logic

- Pass threshold: **61%**
- The API automatically returns a `"passed": true|false|null` field on:
  - `FinishExam`
  - `GET /api/v1/exams`
  - `GET /api/v1/exams/:id`
- `null` means the exam has not yet been finished.

---
