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

def api_request(path, method="GET", body=None):
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
    except Exception as e:
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
                    "description": "Qué se logró hoy (tickets, archivos modificados, bugs corregidos, features implementadas y pruebas que pasaron)."
                },
                "pending": {
                    "type": "string",
                    "description": "Qué queda pendiente para continuar mañana (próximos pasos técnicos claros, commits por hacer, pruebas pendientes)."
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
