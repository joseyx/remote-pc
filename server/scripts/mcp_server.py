#!/usr/bin/env python3
"""
Remote PC - Model Context Protocol (MCP) Server
Allows AI agents (Claude Code, Antigravity CLI) to interact natively with Remote PC Calendar,
Daily Notes, and Standup Handoffs from any project terminal.
"""

import sys
import os
import json
import urllib.request
import urllib.error

# Ensure stdin/stdout are using utf-8 without buffering issues
if sys.stdout.encoding and sys.stdout.encoding.lower() != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

if sys.stdin.encoding and sys.stdin.encoding.lower() != 'utf-8':
    try:
        sys.stdin.reconfigure(encoding='utf-8')
    except Exception:
        pass

SERVER_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOT_DIR = os.path.dirname(SERVER_DIR)

def get_remote_pc_config():
    paths = [
        os.path.join(ROOT_DIR, "server.config.json"),
        os.path.join(r"C:\Users\Jose\Documents\Personal Projects\Remote PC", "server.config.json"),
        os.path.expanduser(r"~\Documents\Personal Projects\Remote PC\server.config.json")
    ]
    for p in paths:
        if os.path.exists(p):
            try:
                with open(p, "r", encoding="utf-8") as f:
                    return json.load(f)
            except Exception:
                pass
    return {"port": "8080", "authToken": ""}

def ensure_server_running(port):
    exe_paths = [
        os.path.join(ROOT_DIR, "remotepc-server.exe"),
        os.path.join(SERVER_DIR, "remotepc-server.exe"),
        r"C:\Users\Jose\Documents\Personal Projects\Remote PC\remotepc-server.exe",
        os.path.expanduser(r"~\Documents\Personal Projects\Remote PC\remotepc-server.exe"),
    ]
    for exe in exe_paths:
        if os.path.exists(exe):
            try:
                import subprocess
                subprocess.Popen(
                    [exe, "-no-open"],
                    cwd=os.path.dirname(exe),
                    creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0x08000000),
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    stdin=subprocess.DEVNULL,
                )
                import time
                url = f"http://127.0.0.1:{port}/api/health"
                for _ in range(15):
                    time.sleep(0.2)
                    try:
                        req = urllib.request.Request(url, method="GET")
                        with urllib.request.urlopen(req, timeout=1) as resp:
                            return True
                    except Exception:
                        pass
                return False
            except Exception:
                pass
    return False

def api_request(path, method="GET", body=None, retried=False):
    cfg = get_remote_pc_config()
    port = cfg.get("port", "8080")
    token = cfg.get("authToken", "")

    url = f"http://127.0.0.1:{port}{path}"
    headers = {
        "Content-Type": "application/json"
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")

    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=6) as resp:
            content = resp.read().decode("utf-8")
            if content:
                return json.loads(content), None
            return {}, None
    except urllib.error.HTTPError as e:
        err_msg = e.read().decode("utf-8")
        return None, f"HTTP {e.code}: {err_msg}"
    except urllib.error.URLError as e:
        if not retried:
            if ensure_server_running(port):
                return api_request(path, method=method, body=body, retried=True)
        return None, f"Error conectando con Remote PC en {url}: {e}. Asegúrate de que remotepc-server esté en ejecución."
    except Exception as e:
        if not retried and ("10061" in str(e) or "refused" in str(e).lower()):
            if ensure_server_running(port):
                return api_request(path, method=method, body=body, retried=True)
        return None, f"Error conectando con Remote PC en {url}: {e}"

# Tool Definitions
TOOLS = [
    {
        "name": "save_daily_handoff",
        "description": (
            "Registra el cierre de tarea / handoff del día actual en el Calendario de Remote PC. "
            "Usa esta herramienta cuando el usuario pida cerrar el día, hacer el handoff, registrar el progreso diario, "
            "o al finalizar la sesión de trabajo. Redacta tú mismo el resumen con base en todo lo avanzado en esta conversación."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "Qué se logró hoy formateado en Markdown estructurado (usa viñetas con saltos de línea '- ...', backticks para `commits/tickets` y negritas; NO un solo párrafo continuo)."
                },
                "pending": {
                    "type": "string",
                    "description": "Qué queda pendiente para continuar mañana formateado en Markdown (usa lista numerada con saltos de línea '1. ...\\n2. ...' o checkboxes '- [ ] ...'; NUNCA un párrafo corrido)."
                },
                "status": {
                    "type": "string",
                    "enum": ["in_progress", "completed", "blocked"],
                    "description": "Estado de la tarea o ticket: in_progress (en curso), completed (finalizado), o blocked (bloqueado)."
                },
                "blockers": {
                    "type": "string",
                    "description": "Bloqueos, dudas de negocio, APIs de terceros caídas o credenciales faltantes si los hay."
                },
                "task_name": {
                    "type": "string",
                    "description": "Nombre de la tarea o ticket (opcional; si se omite se detecta automáticamente por el directorio actual)."
                },
                "agent": {
                    "type": "string",
                    "enum": ["claude", "agy", "developer"],
                    "description": "Agente responsable que redacta el handoff (por defecto claude)."
                }
            },
            "required": ["summary", "pending"]
        }
    },
    {
        "name": "get_standup_pending",
        "description": (
            "Obtiene los puntos pendientes dejados el día anterior o en el último cierre de tareas registrado en Remote PC. "
            "Útil al inicio de la jornada o cuando el usuario pregunta qué quedó pendiente de ayer."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "date": {
                    "type": "string",
                    "description": "Fecha de referencia YYYY-MM-DD (opcional; por defecto la fecha actual)."
                }
            }
        }
    },
    {
        "name": "get_today_note",
        "description": (
            "Consulta la nota diaria y los handoffs registrados en el Calendario de Remote PC para hoy (o una fecha dada)."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "date": {
                    "type": "string",
                    "description": "Fecha YYYY-MM-DD (opcional; por defecto la fecha de hoy)."
                }
            }
        }
    },
    {
        "name": "update_daily_handoff",
        "description": (
            "Modifica o actualiza un handoff existente en el Calendario de Remote PC. "
            "Si no se proporciona id, busca automáticamente el handoff correspondiente al proyecto o tarea actual de hoy."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "id": {
                    "type": "string",
                    "description": "ID del handoff a actualizar (opcional si se puede auto-detectar por la tarea actual)."
                },
                "summary": {
                    "type": "string",
                    "description": "Nuevo resumen de lo completado hoy."
                },
                "pending": {
                    "type": "string",
                    "description": "Nuevas tareas pendientes para mañana."
                },
                "status": {
                    "type": "string",
                    "enum": ["in_progress", "completed", "blocked"],
                    "description": "Nuevo estado de la tarea."
                },
                "blockers": {
                    "type": "string",
                    "description": "Puntos de atención o bloqueos."
                },
                "date": {
                    "type": "string",
                    "description": "Fecha YYYY-MM-DD (opcional; por defecto hoy)."
                }
            }
        }
    },
    {
        "name": "delete_daily_handoff",
        "description": "Elimina un handoff registrado del Calendario de Remote PC.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "id": {
                    "type": "string",
                    "description": "ID del handoff a eliminar."
                },
                "date": {
                    "type": "string",
                    "description": "Fecha YYYY-MM-DD del handoff (opcional)."
                }
            },
            "required": ["id"]
        }
    },
    {
        "name": "add_daily_todo",
        "description": (
            "Agrega una tarea o ítem accionable (con checkbox [ ] o [x]) a la sección de Notas Generales "
            "del Calendario de Remote PC para el día de hoy (o una fecha específica). "
            "Ideal cuando el usuario pide anotar un pendiente rápido o marcar una micro-tarea como lista."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "text": {
                    "type": "string",
                    "description": "Texto del to-do o tarea."
                },
                "checked": {
                    "type": "boolean",
                    "description": "Si la tarea ya está completada (true genera '- [x]', false genera '- [ ]'; por defecto false)."
                },
                "date": {
                    "type": "string",
                    "description": "Fecha YYYY-MM-DD (opcional; por defecto hoy)."
                }
            },
            "required": ["text"]
        }
    },
    {
        "name": "append_daily_note",
        "description": (
            "Anexa texto, notas libres, comandos o apuntes a la nota diaria del Calendario de Remote PC "
            "sin crear un bloque de handoff."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "text": {
                    "type": "string",
                    "description": "Texto o markdown a anexar a la nota diaria."
                },
                "date": {
                    "type": "string",
                    "description": "Fecha YYYY-MM-DD (opcional; por defecto hoy)."
                }
            },
            "required": ["text"]
        }
    },
    {
        "name": "list_workspace_bubbles",
        "description": (
            "Lista todos los workspace bubbles y entornos de tareas (Aditum, Brif, etc.) configurados en Remote PC "
            "con su estado, rama Git actual, puerto de desarrollo y ruta."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "workspace": {
                    "type": "string",
                    "enum": ["all", "aditum", "brif"],
                    "description": "Filtrar por espacio de trabajo (por defecto 'all')."
                },
                "scan": {
                    "type": "boolean",
                    "description": "Reescanear el sistema de archivos para descubrir nuevos worktrees (por defecto true)."
                }
            }
        }
    },
    {
        "name": "get_bubble_context",
        "description": (
            "Obtiene el contexto detallado de una tarea o bubble de Remote PC: rama activa, status de Git, "
            "archivos modificados, puerto dev asignado y schema de base de datos. "
            "Si se omite 'bubble_id', se auto-detecta según el directorio actual."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "bubble_id": {
                    "type": "string",
                    "description": "ID del bubble o nombre de tarea (opcional; se auto-detecta del directorio actual)."
                }
            }
        }
    },
    {
        "name": "create_workspace_bubble",
        "description": (
            "Crea un nuevo workspace bubble y entorno de trabajo (git worktree aislado, branch y configuración) "
            "directamente desde el agente. Soporta espacios como Aditum y Brif."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "workspace": {
                    "type": "string",
                    "enum": ["aditum", "brif"],
                    "description": "Espacio de trabajo donde crear la tarea ('aditum' o 'brif')."
                },
                "task_id": {
                    "type": "string",
                    "description": "Identificador del ticket o tarea (ej: 'TAR-1150' para Aditum, o 'dialogos-mx' para Brif)."
                },
                "name": {
                    "type": "string",
                    "description": "Nombre legible para la tarea (opcional; por defecto igual a task_id)."
                },
                "base_branch": {
                    "type": "string",
                    "description": "Rama base para crear el worktree (opcional; ej: 'development')."
                },
                "use_slim": {
                    "type": "boolean",
                    "description": "Solo para Aditum: usar seed ligera de base de datos (por defecto true)."
                },
                "is_worktree": {
                    "type": "boolean",
                    "description": "Crear como git worktree aislado (true) o branch lineal en el repo principal (false). Por defecto true."
                }
            },
            "required": ["workspace", "task_id"]
        }
    }
]

def handle_save_daily_handoff(args):
    cwd = os.getcwd()
    bubble_id = ""
    task_name = args.get("task_name", "")

    # Auto-detect bubble from cwd
    bubbles, _ = api_request("/api/bubbles")
    if bubbles and isinstance(bubbles, list):
        cwd_norm = os.path.normpath(cwd).lower()
        for b in bubbles:
            b_path = os.path.normpath(b.get("path", "")).lower()
            if b_path and (cwd_norm == b_path or cwd_norm.startswith(b_path)):
                bubble_id = b.get("id")
                if not task_name:
                    task_name = b.get("name")
                break

    if not task_name:
        task_name = os.path.basename(cwd) or "Tarea"

    payload = {
        "bubbleId": bubble_id,
        "workDir": cwd,
        "taskName": task_name,
        "agent": args.get("agent", "claude"),
        "status": args.get("status", "in_progress"),
        "summary": args.get("summary", ""),
        "pending": args.get("pending", ""),
        "blockers": args.get("blockers", "")
    }

    res, err = api_request("/api/calendar/handoff", method="POST", body=payload)
    if err:
        return f"Error al guardar handoff en Remote PC: {err}"

    status_label = "🟡 En Progreso"
    if res.get("status") == "completed":
        status_label = "🟢 Completado"
    elif res.get("status") == "blocked":
        status_label = "🔴 Bloqueado"

    output = [
        f"✓ **Cierre de jornada registrado exitosamente en Remote PC Calendar**",
        f"- **Fecha:** {res.get('date')} ({res.get('time')})",
        f"- **Tarea:** {res.get('taskName')} ({res.get('agent')})",
        f"- **Estado:** {status_label}",
        f"- **Lo completado:** {res.get('summary')}",
        f"- **Pendiente para mañana:** {res.get('pending')}"
    ]
    if res.get("blockers"):
        output.append(f"- **Bloqueos:** {res.get('blockers')}")
    if res.get("branch"):
        output.append(f"- **Git Snapshot:** Rama `{res.get('branch')}` | {res.get('dirtyFiles', 0)} archivo(s) sin commitear")

    return "\n".join(output)

def handle_get_standup_pending(args):
    date = args.get("date", "")
    query = f"?date={date}" if date else ""
    res, err = api_request(f"/api/calendar/standup{query}")
    if err:
        return f"Error al consultar standup: {err}"

    items = res.get("pendingItems", [])
    ref_date = res.get("referenceDate", "")
    if not items:
        return f"No hay pendientes registrados para la fecha de referencia {ref_date or 'reciente'}."

    lines = [f"📋 **Pendientes de standup (Fecha de referencia: {ref_date})**:"]
    for it in items:
        status_icon = "🟡" if it.get("status") == "in_progress" else "🔴"
        lines.append(f"\n### {status_icon} {it.get('taskName')} ({it.get('agent')})")
        if it.get("pending"):
            lines.append(f"- **Pendiente:** {it.get('pending')}")
        if it.get("summary"):
            lines.append(f"- **Último avance:** {it.get('summary')}")
        if it.get("branch"):
            lines.append(f"- **Rama:** `{it.get('branch')}`")

    return "\n".join(lines)

def handle_get_today_note(args):
    date = args.get("date", "")
    query = f"?date={date}" if date else ""
    res, err = api_request(f"/api/calendar/note{query}")
    if err:
        return f"Error al consultar nota diaria: {err}"

    content = res.get("content", "")
    handoffs = res.get("handoffs", [])
    return f"📅 **Nota Diaria ({res.get('date')})**:\n\n{content}\n\n*Handoffs registrados:* {len(handoffs)}"

def handle_update_daily_handoff(args):
    handoff_id = args.get("id")
    date = args.get("date", "")

    if not handoff_id:
        note_res, _ = api_request(f"/api/calendar/note?date={date}" if date else "/api/calendar/note")
        if note_res and note_res.get("handoffs"):
            handoffs = note_res["handoffs"]
            cwd_name = os.path.basename(os.getcwd()).lower()
            matched = None
            for h in handoffs:
                t_name = (h.get("taskName") or "").lower()
                if cwd_name in t_name or t_name in cwd_name:
                    matched = h
                    break
            if not matched and len(handoffs) == 1:
                matched = handoffs[0]
            if matched:
                handoff_id = matched.get("id")
                if not date:
                    date = matched.get("date")

    if not handoff_id:
        return "Error: No se encontró el ID del handoff a actualizar. Especifica el parámetro 'id'."

    payload = {
        "id": handoff_id,
        "date": date
    }
    for field in ["summary", "pending", "status", "blockers"]:
        if field in args:
            payload[field] = args[field]
    if "task_name" in args:
        payload["taskName"] = args["task_name"]

    res, err = api_request("/api/calendar/handoff", method="PUT", body=payload)
    if err:
        return f"Error al actualizar handoff: {err}"
    return f"✓ **Handoff actualizado correctamente** (Tarea: {res.get('taskName')}, Estado: {res.get('status')})"

def handle_delete_daily_handoff(args):
    handoff_id = args.get("id")
    date = args.get("date", "")
    if not handoff_id:
        return "Error: Se requiere el parámetro 'id' para eliminar el handoff."

    query = f"?id={handoff_id}"
    if date:
        query += f"&date={date}"
    res, err = api_request(f"/api/calendar/handoff{query}", method="DELETE")
    if err:
        return f"Error al eliminar handoff: {err}"
    return f"✓ **Handoff eliminado exitosamente** (ID: {handoff_id})"

def handle_add_daily_todo(args):
    text = args.get("text", "").strip()
    if not text:
        return "Error: Se requiere el parámetro 'text' con la descripción del to-do."
    date = args.get("date", "")
    checked = bool(args.get("checked", False))

    payload = {
        "text": text,
        "checked": checked,
        "date": date
    }
    res, err = api_request("/api/calendar/todo", method="POST", body=payload)
    if err:
        return f"Error al agregar to-do en Remote PC: {err}"

    box = "[x]" if checked else "[ ]"
    return (
        f"✓ **To-do registrado en la nota diaria ({res.get('date')})**\n"
        f"- {box} {text}"
    )

def handle_append_daily_note(args):
    text = args.get("text", "").strip()
    if not text:
        return "Error: Se requiere el parámetro 'text' para anexar a la nota diaria."
    date = args.get("date", "")

    payload = {
        "text": text,
        "date": date
    }
    res, err = api_request("/api/calendar/note/append", method="POST", body=payload)
    if err:
        return f"Error al anexar texto en la nota diaria: {err}"

    return f"✓ **Nota diaria actualizada exitosamente ({res.get('date')})**"

def handle_list_workspace_bubbles(args):
    workspace = (args.get("workspace") or "all").lower()
    scan = args.get("scan", True)
    query = "?scan=true" if scan else ""

    bubbles, err = api_request(f"/api/bubbles{query}")
    if err:
        return f"Error al listar bubbles: {err}"

    if not bubbles or not isinstance(bubbles, list):
        return "No hay workspace bubbles registrados en Remote PC."

    if workspace != "all":
        bubbles = [b for b in bubbles if (b.get("workspace") or "").lower() == workspace]

    if not bubbles:
        return f"No se encontraron workspace bubbles para el espacio '{workspace}'."

    lines = [f"🌐 **Workspace Bubbles en Remote PC ({len(bubbles)} encontrados)**:"]
    for b in bubbles:
        status_icon = "🟢" if b.get("status") == "running" or b.get("devRunning") else "⚪"
        ws_tag = f"[{b.get('workspace', 'general').upper()}]"
        name = b.get("name") or b.get("id")
        branch = b.get("branch") or "sin rama"
        port_info = f" | Puerto :{b.get('port')}" if b.get("port") else ""
        schema_info = f" | DB: `{b.get('dbSchema')}`" if b.get("dbSchema") else ""
        worktree_badge = " (Worktree)" if b.get("isWorktree") else ""

        lines.append(f"\n- {status_icon} **{name}** {ws_tag}{worktree_badge}")
        lines.append(f"  - **ID:** `{b.get('id')}` | **Rama:** `{branch}`{port_info}{schema_info}")
        lines.append(f"  - **Ruta:** `{b.get('path')}`")

    return "\n".join(lines)

def handle_get_bubble_context(args):
    bubble_id = args.get("bubble_id", "").strip()
    bubbles, err = api_request("/api/bubbles")
    if err:
        return f"Error al consultar bubbles: {err}"

    matched = None
    if bubble_id:
        for b in (bubbles or []):
            if b.get("id") == bubble_id or bubble_id.lower() in (b.get("name") or "").lower():
                matched = b
                break
    else:
        cwd = os.getcwd()
        cwd_norm = os.path.normpath(cwd).lower()
        for b in (bubbles or []):
            b_path = os.path.normpath(b.get("path", "")).lower()
            if b_path and (cwd_norm == b_path or cwd_norm.startswith(b_path)):
                matched = b
                break

    if not matched:
        available = ", ".join([f"`{b.get('id')}`" for b in (bubbles or [])[:10]])
        return (
            "No se pudo determinar el bubble actual a partir del directorio ni del parámetro 'bubble_id'.\n"
            f"Bubbles disponibles: {available}"
        )

    b_id = matched.get("id")
    git_st, _ = api_request(f"/api/bubbles/{b_id}/git/status")

    lines = [
        f"🎯 **Contexto del Bubble:** `{matched.get('name')}` (`{b_id}`)",
        f"- **Espacio:** {matched.get('workspace', '').upper()}",
        f"- **Ruta de trabajo:** `{matched.get('path')}`",
        f"- **Puerto dev:** {matched.get('port') or 'No asignado'}",
    ]
    if matched.get("dbSchema"):
        lines.append(f"- **Esquema DB:** `{matched.get('dbSchema')}`")
    if matched.get("priority"):
        lines.append(f"- **Prioridad:** {matched.get('priority')}")

    if git_st and isinstance(git_st, dict):
        branch = git_st.get("branch") or matched.get("branch")
        is_dirty = git_st.get("isDirty", False)
        ahead = git_st.get("ahead", 0)
        behind = git_st.get("behind", 0)
        changes = git_st.get("changes", [])

        lines.append("\n**Estado Git:**")
        lines.append(f"- **Rama:** `{branch}`")
        lines.append(f"- **Commits:** Ahead: {ahead} | Behind: {behind}")
        lines.append(f"- **Estado de trabajo:** {'⚠️ Cambios sin commitear' if is_dirty else '✨ Directorio limpio'}")

        if changes:
            lines.append(f"- **Archivos modificados ({len(changes)}):**")
            for c in changes[:8]:
                st_flag = c.get("status", "M")
                staged = " (staged)" if c.get("staged") else ""
                lines.append(f"  - `[{st_flag}]` {c.get('path')}{staged}")
            if len(changes) > 8:
                lines.append(f"  - ... y {len(changes) - 8} archivo(s) más")
    else:
        lines.append(f"- **Rama actual:** `{matched.get('branch')}`")

    return "\n".join(lines)

def handle_create_workspace_bubble(args):
    workspace = (args.get("workspace") or "").strip().lower()
    task_id = (args.get("task_id") or "").strip()
    if not workspace or not task_id:
        return "Error: Se requieren 'workspace' ('aditum' o 'brif') y 'task_id' (ej: 'TAR-1150')."

    payload = {
        "workspace": workspace,
        "taskId": task_id,
        "name": args.get("name") or task_id,
        "baseBranch": args.get("base_branch", ""),
        "useSlim": args.get("use_slim", True),
        "isWorktree": args.get("is_worktree", True)
    }

    res, err = api_request("/api/bubbles", method="POST", body=payload)
    if err:
        return f"Error al crear bubble en Remote PC: {err}"

    lines = [
        "🎉 **Workspace Bubble creado y configurado exitosamente**",
        f"- **Tarea:** {res.get('name') or task_id}",
        f"- **ID:** `{res.get('id')}`",
        f"- **Espacio:** {res.get('workspace', '').upper()}",
        f"- **Ruta:** `{res.get('path')}`",
        f"- **Rama:** `{res.get('branch')}`",
    ]
    if res.get("port"):
        lines.append(f"- **Puerto dev:** {res.get('port')}")
    if res.get("dbSchema"):
        lines.append(f"- **Esquema DB:** `{res.get('dbSchema')}`")

    return "\n".join(lines)

def execute_tool(name, arguments):
    if name == "save_daily_handoff":
        return handle_save_daily_handoff(arguments or {})
    elif name == "get_standup_pending":
        return handle_get_standup_pending(arguments or {})
    elif name == "get_today_note":
        return handle_get_today_note(arguments or {})
    elif name == "update_daily_handoff":
        return handle_update_daily_handoff(arguments or {})
    elif name == "delete_daily_handoff":
        return handle_delete_daily_handoff(arguments or {})
    elif name == "add_daily_todo":
        return handle_add_daily_todo(arguments or {})
    elif name == "append_daily_note":
        return handle_append_daily_note(arguments or {})
    elif name == "list_workspace_bubbles":
        return handle_list_workspace_bubbles(arguments or {})
    elif name == "get_bubble_context":
        return handle_get_bubble_context(arguments or {})
    elif name == "create_workspace_bubble":
        return handle_create_workspace_bubble(arguments or {})
    else:
        raise ValueError(f"Herramienta desconocida: {name}")

def main():
    while True:
        try:
            line = sys.stdin.readline()
            if not line:
                break
            line = line.strip()
            if not line:
                continue

            msg = json.loads(line)
            msg_id = msg.get("id")
            method = msg.get("method")
            params = msg.get("params", {})

            if method == "initialize":
                resp = {
                    "jsonrpc": "2.0",
                    "id": msg_id,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "capabilities": {
                            "tools": {}
                        },
                        "serverInfo": {
                            "name": "remote-pc-calendar",
                            "version": "1.0.0"
                        }
                    }
                }
                sys.stdout.write(json.dumps(resp) + "\n")
                sys.stdout.flush()

            elif method == "notifications/initialized":
                # Notification, no response needed
                pass

            elif method == "ping":
                resp = {
                    "jsonrpc": "2.0",
                    "id": msg_id,
                    "result": {}
                }
                sys.stdout.write(json.dumps(resp) + "\n")
                sys.stdout.flush()

            elif method == "tools/list":
                resp = {
                    "jsonrpc": "2.0",
                    "id": msg_id,
                    "result": {
                        "tools": TOOLS
                    }
                }
                sys.stdout.write(json.dumps(resp) + "\n")
                sys.stdout.flush()

            elif method == "tools/call":
                tool_name = params.get("name")
                tool_args = params.get("arguments", {})
                try:
                    res_text = execute_tool(tool_name, tool_args)
                    resp = {
                        "jsonrpc": "2.0",
                        "id": msg_id,
                        "result": {
                            "content": [
                                {
                                    "type": "text",
                                    "text": res_text
                                }
                            ],
                            "isError": False
                        }
                    }
                except Exception as ex:
                    resp = {
                        "jsonrpc": "2.0",
                        "id": msg_id,
                        "result": {
                            "content": [
                                {
                                    "type": "text",
                                    "text": f"Error ejecutando {tool_name}: {ex}"
                                }
                            ],
                            "isError": True
                        }
                    }
                sys.stdout.write(json.dumps(resp) + "\n")
                sys.stdout.flush()

            else:
                # Unknown method
                if msg_id is not None:
                    resp = {
                        "jsonrpc": "2.0",
                        "id": msg_id,
                        "error": {
                            "code": -32601,
                            "message": f"Method not found: {method}"
                        }
                    }
                    sys.stdout.write(json.dumps(resp) + "\n")
                    sys.stdout.flush()

        except (IOError, KeyboardInterrupt):
            break
        except Exception as e:
            # Send JSON-RPC error if possible
            sys.stderr.write(f"MCP Server error: {e}\n")
            sys.stderr.flush()

if __name__ == "__main__":
    main()
