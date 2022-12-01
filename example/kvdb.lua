local db = netcode.kvdb('app.db')
local ret = db.set('users', 'netcode', 'yes!!!')
local val, ok = db.get('users', 'netcode')

print('set:', ret)
print('get:', ok, val)

db.close()

netcode.exit()