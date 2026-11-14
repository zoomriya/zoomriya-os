# The assistant contract

How an assistant answers in Box Desktop's Myira panel ([Box Desktop §4](box-desktop.md#myira)).
The OS's own Myira answers it inside `box-desktopd`; the Myira app, when it is installed
and on for the admin, answers it with its own API. The panel shows either the same way.
Nothing of the app's code is shared with the OS: they meet only here, on loopback.

## Who answers

| Case | Who |
| --- | --- |
| The Myira app answers `GET {app}/me` with 200 for the admin's token | The Myira app |
| Anything else: not installed, off for this admin, not answering | The OS's own Myira (the box only) |

`box-desktopd` asks before each question, so installing, removing or switching off the
app changes the panel at once. `{app}` is `http://127.0.0.1:8300/myira/api/v1`
(`BOX_DESKTOP_MYIRA_APP`).

## Requests

`box-desktopd` sends each as the signed-in admin: `Authorization: Bearer` with their
ZOOMRIYA ID token, renewed once if it ran out.

| Request | Body | Answer |
| --- | --- | --- |
| `GET {app}/me` | | 200 when the app is on for this member |
| `POST {app}/ask` | `question`, `lang` (`en`, `fr`), `conversation_id` (or null for a new chat), `tz`, `tools` (the OS's tools, below), `surface` (`box-desktop`: a new chat is marked with Box Desktop's sign in History) | A stream |
| `POST {app}/ask/tool` | `conversation_id`, `question_id`, `app`, `name`, `title`, `arguments`, `result` (the tool's answer, as text), `is_error`, `declined`, `lang`, `earlier`, `tools`, `tz` | A stream: the answer from the tool's result |
| `GET {app}/conversations` | | The admin's chats: `id`, `title`, `mode`, `pinned`, `updated_at` |
| `GET {app}/conversations/{id}` | | One chat: `id`, `title`, `turns` (`role`, `content`, …) |
| `DELETE {app}/conversations/{id}` | | 204 |

## The stream

One JSON object a line (`application/x-ndjson`), in this order:

| `type` | Fields | Meaning |
| --- | --- | --- |
| `route` | `conversation_id`, `question_id`, `mode`, `fixed` | First: the chat the answer belongs to (null for the OS's own Myira, which keeps none) |
| `text` | `text` | A piece of the answer, as it is written |
| `step` | `app`, `name`, `title`, `arguments`, `result`, `is_error`, `declined` | From `box-desktopd`, first in the answer to a tool: what came of it, which the panel sends back as `earlier` if Myira picks another tool for the same request (at most three) |
| `tool` | `app`, `name`, `title`, `arguments`, `writes` | A tool picked: the panel has `box-desktopd` run it, after the admin confirms a change on its card, then sends what came of it to `ask/tool` |
| `done` | `answer`, `choices`, `tool`, and from the OS's own Myira `say`, `source`, `open` | Last |
| `error` | `detail`, or `say` | The answer stopped |

`say` is a fixed reply's code, worded by the page in the admin's language:

| `say` | The words |
| --- | --- |
| `not_box` | "For now I can help with the box. With the ZOOMRIYA apps installed, I can answer much more." |
| `no_model` | "Myira needs her model to answer (4.4 GB)." |
| `unavailable` | "Myira can't answer right now. Try again in a moment." |
| `declined` | "Nothing was changed." |
| `needs_password` | "That asks for your password again, so it's done in its own place. Here it is." (with `open`) |
| `emergency` | "If someone is hurt or in danger, call {number} now." with `number`, or without one "…call your local emergency number now." |

`source` is a Help topic's name ("From Help: …"), and `open` its tool and section, shown
as a button.

## The OS's tools

Offered with every question from the panel, and nowhere else (not on `/myira`, not to AI
apps through `/mcp`). Each in the apps' tool shape (contract 11): `app` (always `box`),
`name`, `title`, `description`, `input_schema`, `writes`. `box-desktopd` runs a tool only
if it is on this list, as the admin; a change only after Confirm on its card.

| Tool | Reads or changes | What |
| --- | --- | --- |
| `box_status` | Reads | The OS's version and how long it gets security fixes |
| `storage` | Reads | How full the library and the system are |
| `drive_health` | Reads | Each drive's health, temperature and age |
| `backups` | Reads | Backups: set up, where, the last one, nightly |
| `security` | Reads | Security fixes, the firewall, the certificate |
| `restore_points` | Reads | The OS's restore points |
| `network` | Reads | The box's address, cable or Wi-Fi, router and DNS |
| `resources` | Reads | Processor, memory, disk and each app now |
| `maintenance_hour` | Reads | When the box restarts by itself for updates |
| `sound` | Reads | Speakers, microphone and their volume |
| `set_maintenance_hour` | Changes | Moves the maintenance hour (`hour`, HH:MM) |
| `backup_now` | Changes | Starts a backup |
| `check_updates_now` | Changes | Checks for and installs security fixes now |
| `set_volume` | Changes | The speakers' volume (`percent`, 0 to 100) |

Never a tool: anything that asks for the password again ([Box Desktop §7](box-desktop.md)):
Erase everything, Reset, Restore, Restart, Shut down, advanced access, SSH keys, making or
removing an admin, turning an account off. `box-desktopd` refuses those even if asked.

## The panel's own routes

What the page asks `box-desktopd` (signed in, the page's token on each change):

| Route | What |
| --- | --- |
| `GET /api/myira` | `who` (`app` or `os`), `chats` (whether chats are kept), `model` (whether the model is loaded) |
| `POST /api/myira/ask` | `question`, `lang`, `conversation_id`, `tz`, `help` (Support Center's Help topics, in the admin's language, for the OS's own Myira): the stream |
| `POST /api/myira/tool` | The picked tool, `declined`, and the question's `conversation_id` and `question_id`: `box-desktopd` runs it, then the stream |
| `GET /api/myira/chats`, `GET` and `DELETE /api/myira/chats/{id}` | The admin's chats, from the app; none from the OS's own Myira |
