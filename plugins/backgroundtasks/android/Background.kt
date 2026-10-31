package space.nimbusgo.nativekit

import android.content.Context
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.Worker
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/** Runs a task's Go, starting it first when the system launched the app with no screen. */
class BackgroundWorker(c: Context, p: WorkerParameters) : Worker(c, p) {
    override fun doWork(): Result {
        val name = inputData.getString("name") ?: return Result.failure()
        BackgroundPlugin.startGo(applicationContext)
        // A repeating task and a one-off of the same name can land together: they share one run.
        var first = false
        val wait = BackgroundPlugin.waiting.computeIfAbsent(name) {
            first = true
            CountDownLatch(1)
        }
        if (first) Bridge.send(Render.encode(Render.BackgroundRun(name = name)))
        val finished = wait.await(30, TimeUnit.SECONDS)
        return if (finished && BackgroundPlugin.results[name] == true) Result.success() else Result.retry()
    }
}

/** backgroundtasks: WorkManager. */
internal object BackgroundPlugin {
    private const val PREFS = "nimbus.background"
    internal val waiting = ConcurrentHashMap<String, CountDownLatch>()
    internal val results = ConcurrentHashMap<String, Boolean>()

    /** A worker can outlive the screen and needs the schema to start Go without one. */
    fun boot() {
        if (Plugins.schema == 0L) return // started with no screen: it's saved already
        Plugins.context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putLong("schema", Plugins.schema).apply()
    }

    @Synchronized
    internal fun startGo(c: Context) {
        val schema = c.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getLong("schema", 0)
        Plugins.context = c.applicationContext
        Plugins.boot()
        // With a screen, the Renderer's handlers are already in place.
        if (!Bridge.hasHost()) {
            Bridge.setHostHandler { req ->
                val m = Render.decode(req)
                if (m is Render.PluginCall) Render.encode(Plugins.start(m)) else null
            }
            Bridge.start(schema)
        }
    }

    fun handle(call: PluginCall) {
        val wm = WorkManager.getInstance(Plugins.context)
        when (call.method) {
            "schedule" -> {
                val n = call.string("name") ?: return call.reject(Plugins.FAILED, "a task needs a name")
                val minutes = (call.args.optDouble("seconds", 3600.0) / 60).toLong().coerceAtLeast(15)
                val w = PeriodicWorkRequestBuilder<BackgroundWorker>(minutes, TimeUnit.MINUTES).setInputData(workDataOf("name" to n)).build()
                wm.enqueueUniquePeriodicWork("nimbus-$n", ExistingPeriodicWorkPolicy.UPDATE, w)
                call.resolve()
            }
            "once" -> {
                val n = call.string("name") ?: return call.reject(Plugins.FAILED, "a task needs a name")
                val w = OneTimeWorkRequestBuilder<BackgroundWorker>().setInputData(workDataOf("name" to n))
                    .setInitialDelay((call.args.optDouble("seconds", 0.0) * 1000).toLong(), TimeUnit.MILLISECONDS).build()
                wm.enqueueUniqueWork("nimbus-once-$n", ExistingWorkPolicy.REPLACE, w)
                call.resolve()
            }
            "cancel" -> {
                wm.cancelUniqueWork("nimbus-${call.string("name")}")
                wm.cancelUniqueWork("nimbus-once-${call.string("name")}")
                call.resolve()
            }
            "done" -> {
                val n = call.string("name") ?: ""
                results[n] = call.args.optBoolean("ok")
                waiting.remove(n)?.countDown()
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
