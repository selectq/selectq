import sqlite3

conn = sqlite3.connect(r"c:\AnvitaPython\gst\bank_statements.db")
cur = conn.cursor()

print("=== REMAINING UNCLASSIFIED ===")
for i, r in enumerate(cur.execute(
    "SELECT txn_date, withdrawal_amt, deposit_amt, narration FROM bank_transactions WHERE account_head = '' ORDER BY id"
)):
    print(f"  {i+1:3d} {r[0]:<10} {r[1]:>12.2f} {r[2]:>12.2f}  {r[3]}")

count = cur.execute("SELECT COUNT(*) FROM bank_transactions WHERE account_head = ''").fetchone()[0]
print(f"\nTotal unclassified: {count}")
conn.close()
