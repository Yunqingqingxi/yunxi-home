import sqlite3
conn = sqlite3.connect("data/yunxi-home.db")
cursor = conn.execute("SELECT name FROM sqlite_master WHERE type='table'")
print([r[0] for r in cursor.fetchall()])
conn.close()
