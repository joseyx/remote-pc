#!/usr/bin/env python3
import sys
import os
import json
import argparse
import urllib.request
import urllib.error

# Ensure UTF-8 output
if sys.stdout.encoding and sys.stdout.encoding.lower() != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

def find_server_config():
    paths = [
        os.path.join(r"C:\Users\Jose\Documents\Personal Projects\Remote PC", "server.config.json"),
        os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "server.config.json"),
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

def main():
    parser = argparse.ArgumentParser(description="Registrar handoff de cierre de jornada en Remote PC Calendar")
    parser.add_argument("--status", choices=["in_progress", "completed", "blocked"], default="in_progress", help="Estado del ticket")
    parser.add_argument("--summary", required=True, help="Qué se logró hoy / avances completados")
    parser.add_argument("--pending", required=True, help="Qué queda pendiente para mañana / próximos pasos")
    parser.add_argument("--blockers", default="", help="Bloqueos o puntos de atención")
    parser.add_argument("--agent", default="claude", choices=["claude", "agy", "developer"], help="Agente responsable")
    parser.add_argument("--bubbleId", default="", help="ID del bubble (opcional, se auto-detecta por directorio)")
    parser.add_argument("--task", default="", help="Nombre de la tarea (opcional)")
    parser.add_argument("--date", default="", help="Fecha YYYY-MM-DD (opcional, defecto hoy)")

    args = parser.parse_args()

    cfg = find_server_config()
    port = cfg.get("port", "8080")
    token = cfg.get("authToken", "")

    headers = {
        "Content-Type": "application/json"
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    cwd = os.getcwd()
    bubble_id = args.bubbleId
    task_name = args.task

    # Auto-detect bubble from current directory if not specified
    if not bubble_id:
        try:
            req = urllib.request.Request(f"http://127.0.0.1:{port}/api/bubbles", headers=headers)
            with urllib.request.urlopen(req, timeout=4) as r:
                bubbles = json.loads(r.read())
                cwd_norm = os.path.normpath(cwd).lower()
                for b in bubbles:
                    b_path = os.path.normpath(b.get("path", "")).lower()
                    if b_path and (cwd_norm == b_path or cwd_norm.startswith(b_path)):
                        bubble_id = b.get("id")
                        if not task_name:
                            task_name = b.get("name")
                        break
        except Exception:
            pass

    if not task_name:
        task_name = os.path.basename(cwd) or "Tarea"

    payload = {
        "bubbleId": bubble_id,
        "taskName": task_name,
        "agent": args.agent,
        "status": args.status,
        "summary": args.summary,
        "pending": args.pending,
        "blockers": args.blockers
    }
    if args.date:
        payload["date"] = args.date

    data = json.dumps(payload).encode("utf-8")
    url = f"http://127.0.0.1:{port}/api/calendar/handoff"

    try:
        req = urllib.request.Request(url, data=data, headers=headers, method="POST")
        with urllib.request.urlopen(req, timeout=6) as resp:
            if resp.status in (200, 201):
                res_data = json.loads(resp.read())
                print(f"✓ Handoff registrado exitosamente en Remote PC Calendar")
                print(f"  Tarea:   {res_data.get('taskName')} ({res_data.get('agent')})")
                print(f"  Estado:  {res_data.get('status')}")
                if res_data.get('branch'):
                    print(f"  Git:     Rama {res_data.get('branch')} | {res_data.get('dirtyFiles', 0)} archivos pendientes")
                return 0
            else:
                print(f"Error: Servidor respondió {resp.status}")
                return 1
    except urllib.error.URLError as e:
        print(f"Error conectando con Remote PC (http://localhost:{port}): {e}")
        return 1

if __name__ == "__main__":
    sys.exit(main())
