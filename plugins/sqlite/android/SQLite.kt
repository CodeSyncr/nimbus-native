package space.nimbusgo.nativekit

import android.database.Cursor
import android.database.sqlite.SQLiteCursor
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteException
import android.database.sqlite.SQLiteProgram
import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.util.UUID
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

/**
 * sqlite: Android's own SQLite. Each open database has a thread of its own: Android ties a
 * transaction to the thread that began it, and Go's database/sql keeps one connection, so every
 * statement for it runs there, in order.
 */
internal object SQLitePlugin {
    private class Handle(val db: SQLiteDatabase, val thread: ExecutorService)

    private val open = HashMap<String, Handle>()

    /** A plain file name: databases stay in the app's own folder. */
    private fun file(name: String?): File? {
        if (name.isNullOrEmpty() || name.contains('/') || name == "." || name == "..") return null
        return Plugins.context.getDatabasePath(name).also { it.parentFile?.mkdirs() }
    }

    private fun bind(p: SQLiteProgram, args: JSONArray?) {
        if (args == null) return
        for (n in 0 until args.length()) {
            val at = n + 1
            when (val v = args.get(n)) {
                JSONObject.NULL -> p.bindNull(at)
                is String -> p.bindString(at, v)
                is Number -> p.bindDouble(at, v.toDouble())
                is Boolean -> p.bindLong(at, if (v) 1 else 0)
                is JSONObject -> when {
                    v.has("i") -> p.bindLong(at, v.getString("i").toLong())
                    v.has("b") -> p.bindBlob(at, Base64.decode(v.getString("b"), Base64.DEFAULT))
                    else -> throw IllegalArgumentException("a strange value for parameter $at")
                }
                else -> throw IllegalArgumentException("a strange value for parameter $at")
            }
        }
    }

    private fun cell(c: Cursor, i: Int): Any = when (c.getType(i)) {
        Cursor.FIELD_TYPE_NULL -> JSONObject.NULL
        Cursor.FIELD_TYPE_INTEGER -> JSONObject().put("i", c.getLong(i).toString())
        Cursor.FIELD_TYPE_FLOAT -> c.getDouble(i)
        Cursor.FIELD_TYPE_BLOB -> JSONObject().put("b", Base64.encodeToString(c.getBlob(i), Base64.NO_WRAP))
        else -> c.getString(i)
    }

    private fun exec(db: SQLiteDatabase, call: PluginCall): Any {
        // BEGIN, COMMIT and ROLLBACK go through Android's own transactions here.
        val changes = db.compileStatement(call.string("sql") ?: "").use { s ->
            bind(s, call.args.optJSONArray("args"))
            s.executeUpdateDelete()
        }
        val last = db.compileStatement("SELECT last_insert_rowid()").use { it.simpleQueryForLong() }
        return JSONObject().put("changes", changes).put("lastId", JSONObject().put("i", last.toString()))
    }

    private fun query(db: SQLiteDatabase, call: PluginCall): Any {
        val args = call.args.optJSONArray("args")
        val factory = SQLiteDatabase.CursorFactory { _, driver, table, q ->
            bind(q, args)
            SQLiteCursor(driver, table, q)
        }
        db.rawQueryWithFactory(factory, call.string("sql") ?: "", null, null).use { c ->
            val rows = JSONArray()
            while (c.moveToNext()) {
                val row = JSONArray()
                for (i in 0 until c.columnCount) row.put(cell(c, i))
                rows.put(row)
            }
            return JSONObject().put("columns", JSONArray(c.columnNames.toList())).put("rows", rows)
        }
    }

    /** Runs on the database's thread and answers from there. */
    private fun on(call: PluginCall, work: (SQLiteDatabase) -> Any?) {
        val h = synchronized(open) { open[call.string("id")] } ?: return call.reject(Plugins.NOT_FOUND, "that database is closed")
        h.thread.execute {
            try {
                call.resolve(work(h.db))
            } catch (e: Exception) { // SQLite's refusals, and a value it couldn't take
                call.reject(Plugins.FAILED, e.message ?: e.toString())
            }
        }
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "open" -> {
                val name = call.string("name")
                val db = try {
                    if (name == ":memory:") SQLiteDatabase.create(null)
                    else SQLiteDatabase.openOrCreateDatabase(file(name) ?: return call.reject(Plugins.FAILED, "a database name is a plain file name"), null)
                } catch (e: SQLiteException) {
                    return call.reject(Plugins.FAILED, e.message ?: e.toString())
                }
                val id = UUID.randomUUID().toString()
                synchronized(open) { open[id] = Handle(db, Executors.newSingleThreadExecutor()) }
                call.resolve(mapOf("id" to id))
            }
            "exec" -> on(call) { exec(it, call) }
            "query" -> on(call) { query(it, call) }
            "close" -> {
                val h = synchronized(open) { open.remove(call.string("id")) } ?: return call.resolve()
                h.thread.execute { h.db.close() }
                h.thread.shutdown()
                call.resolve()
            }
            "delete" -> {
                val f = file(call.string("name")) ?: return call.reject(Plugins.FAILED, "a database name is a plain file name")
                SQLiteDatabase.deleteDatabase(f)
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
