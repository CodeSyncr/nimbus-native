package space.nimbusgo.hello

import android.app.Activity
import android.graphics.Typeface
import android.os.Bundle
import android.os.SystemClock
import android.util.Log
import android.widget.ScrollView
import android.widget.TextView
import java.io.File
import space.nimbusgo.nativekit.Bridge
import space.nimbusgo.nativekit.BridgeException
import space.nimbusgo.nativekit.WireException

/**
 * Bridge and wire checks, run at launch. Everything crosses as generated Proto messages. Every line
 * also goes to logcat (tag NimbusNativeApp).
 */
class MainActivity : Activity() {
    private lateinit var text: TextView
    private val lines = StringBuilder()
    private var stage = PING
    private var ticks = 0
    private var pings = 0
    private var burstNext = 0
    private var t0 = 0L
    private var drainsBefore = 0L
    private var soakOK = false
    private var soakA = LongArray(0) // heap, rss, goroutines after round 1

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        text = TextView(this).apply {
            typeface = Typeface.MONOSPACE
            textSize = 11f
            val pad = (16 * resources.displayMetrics.density).toInt()
            setPadding(pad, pad * 3, pad, pad)
        }
        setContentView(ScrollView(this).apply { addView(text) })

        Bridge.setListener { bytes ->
            try {
                received(Proto.decode(bytes))
            } catch (e: WireException) {
                fail("undecodable message from Go: ${e.message}")
            }
        }
        try {
            run()
        } catch (e: RuntimeException) {
            fail(e.toString())
        }
    }

    private fun run() {
        try {
            Bridge.start(NimbusSchema.COMBINED xor 1)
            return fail("a stale schema was accepted")
        } catch (e: BridgeException) {
            if (e.code != Bridge.ERR_SCHEMA) throw e
            log("schema guard: stale hash refused")
        }

        t0 = SystemClock.elapsedRealtimeNanos()
        Bridge.start(NimbusSchema.COMBINED)
        log("start: ok in %.2f ms, schema %x".format(since(t0) / 1e3, NimbusSchema.COMBINED))

        val g = call(Proto.Hello(name = "Yash")) as Proto.Greeting
        log("call: ${g.text} ${g.goVersion} on ${g.platform}")

        val n = 10_000
        val echo = Proto.Echo(text = "x")
        val b0 = SystemClock.elapsedRealtimeNanos()
        repeat(n) { call(echo) }
        log("typed sync calls: %d, avg %.2f µs (encode + call + decode)".format(n, since(b0) / n))

        try {
            call(Proto.Crash(reason = "asked to panic"))
            return fail("panic not reported")
        } catch (e: BridgeException) {
            val stack = e.message!!.contains("asked to panic") && e.message!!.contains("goroutine")
            log("panic: recovered (code ${e.code}), message + Go stack ${if (stack) "delivered" else "MISSING"}")
        }

        var rows = 0
        val s0 = SystemClock.elapsedRealtimeNanos()
        repeat(20) {
            val s = call(Proto.SnapshotRequest(rows = 1000)) as Proto.Snapshot
            rows += s.rows.size
            if (s.rows[2].subtitle == null || s.rows[1].subtitle != null ||
                s.rows[999].attrs["city"] != "Pune" || s.rows[5].kind != Proto.Kind.BUTTON
            ) return fail("snapshot fields wrong")
        }
        log("snapshot: %d rows decoded, %.2f µs/row (Go encode + copy + Kotlin decode)".format(rows, since(s0) / rows))

        t0 = SystemClock.elapsedRealtimeNanos()
        send(Proto.Ping(seq = 0))
    }

    private fun received(m: Proto.Message) {
        when {
            m is Proto.Ready -> log("go → native: ready (${m.goVersion})")
            m is Proto.Tick -> {
                ticks++
                log("go → native: tick ${m.n} (goroutine)")
                maybePass()
            }
            stage == PING && m is Proto.Pong -> {
                if (m.seq != pings.toLong()) return fail("pong ${m.seq}, want $pings")
                if (++pings < PING_COUNT) {
                    send(Proto.Ping(seq = pings.toLong()))
                } else {
                    log("ping-pong: %d typed round trips, avg %.2f µs".format(PING_COUNT, since(t0) / PING_COUNT))
                    startBurst(BURST)
                }
            }
            stage in BURST..SOAK2 && m is Proto.Item -> {
                if (m.index != burstNext.toLong()) return fail("burst out of order: ${m.index}, want $burstNext")
                if (++burstNext == BURST_COUNT) burstDone()
            }
            stage != DONE -> fail("unexpected message ${m.javaClass.simpleName}")
        }
    }

    private fun startBurst(next: Int) {
        stage = next
        burstNext = 0
        drainsBefore = stats(false).drains
        t0 = SystemClock.elapsedRealtimeNanos()
        call(Proto.StartBurst(count = BURST_COUNT.toLong()))
    }

    private fun burstDone() {
        val drains = stats(false).drains - drainsBefore
        if (stage == BURST) {
            log("burst: %d typed msgs in order, %d drains, %.0f ns/msg".format(BURST_COUNT, drains, since(t0) * 1000 / BURST_COUNT))
            return soakRound(SOAK1)
        }
        val s = stats(true)
        val mem = longArrayOf(s.heap, rss(), s.goroutines)
        if (stage == SOAK1) {
            soakA = mem
            return soakRound(SOAK2)
        }
        log("soak: 2 rounds × (%d calls + %d msgs)".format(SOAK_CALLS, BURST_COUNT))
        log("  Go heap %d → %d KB, RSS %d → %d MB, goroutines %d → %d".format(
            soakA[0] / 1024, mem[0] / 1024, soakA[1] shr 20, mem[1] shr 20, soakA[2], mem[2]))
        if (mem[0] - soakA[0] > 512 * 1024 || mem[1] - soakA[1] > 4 shl 20 || mem[2] > soakA[2] + 2) {
            return fail("memory grew between soak rounds")
        }
        log("soak: no leak")
        soakOK = true
        stage = DONE
        maybePass()
    }

    private fun soakRound(next: Int) {
        val echo = Proto.Echo(text = "soak")
        repeat(SOAK_CALLS) { call(echo) }
        startBurst(next)
    }

    private fun maybePass() {
        if (soakOK && ticks == 3) log("NN-0C RESULT: PASS")
    }

    private fun call(m: Proto.Message): Proto.Message = Proto.decode(Bridge.call(Proto.encode(m)))

    private fun send(m: Proto.Message) = Bridge.send(Proto.encode(m))

    private fun stats(gc: Boolean) = call(Proto.StatsRequest(gc = gc)) as Proto.Stats

    /** Resident set size in bytes, from /proc/self/statm. */
    private fun rss(): Long = try {
        File("/proc/self/statm").readText().split(" ")[1].toLong() * 4096
    } catch (e: Exception) {
        0
    }

    private fun since(t: Long) = (SystemClock.elapsedRealtimeNanos() - t) / 1e3

    private fun fail(s: String) {
        stage = DONE
        log("FAIL: $s")
    }

    private fun log(s: String) {
        Log.i("NimbusNativeApp", "[hello] $s")
        lines.append(s).append('\n')
        text.text = "Nimbus Native · bridge + wire checks\n\n$lines"
    }

    private companion object {
        const val PING_COUNT = 1000
        const val BURST_COUNT = 100_000
        const val SOAK_CALLS = 100_000
        const val PING = 0
        const val BURST = 1
        const val SOAK1 = 2
        const val SOAK2 = 3
        const val DONE = 4
    }
}
