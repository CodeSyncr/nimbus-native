package space.nimbusgo.nativekit;

import android.content.Context;
import android.content.res.ColorStateList;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Paint;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.text.Editable;
import android.text.InputType;
import android.text.Layout;
import android.text.StaticLayout;
import android.text.TextPaint;
import android.text.TextUtils;
import android.text.TextWatcher;
import android.util.LruCache;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.EditText;
import android.widget.ImageView;
import android.widget.ProgressBar;
import android.widget.TextView;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Applies the Go runtime's commits to Android views and reports taps, edits and the viewport
 * back. Lengths arrive in dp and are kept as px frames.
 */
public final class Renderer {
    /** After each commit is applied: the commit and how long applying took, in ms. */
    public interface CommitListener {
        void onCommit(Render.Commit c, double applyMs);
    }

    public CommitListener onCommit;

    final Context ctx;
    final float density;
    final NimbusView root;
    private final Map<Integer, View> views = new HashMap<>();
    private final Map<Integer, int[]> frames = new HashMap<>(); // l, t, w, h in px
    private final Map<View, String> pendingTitles = new HashMap<>(); // titles for screens not in a stack yet
    private int rootTag;
    private Render.Viewport lastViewport;
    private boolean started;

    private static final int[] NO_FRAME = {0, 0, 0, 0};

    Renderer(NimbusView root) {
        this.root = root;
        this.ctx = root.getContext();
        this.density = ctx.getResources().getDisplayMetrics().density;
    }

    void start(long schema) {
        Bridge.setListener(bytes -> {
            Render.Message m;
            try {
                m = Render.decode(bytes);
            } catch (WireException e) {
                android.util.Log.e("NimbusNative", "undecodable commit", e);
                return;
            }
            if (m instanceof Render.Commit) apply((Render.Commit) m);
        });
        float d = density;
        Bridge.setHostHandler(req -> answer(req, d));
        Bridge.start(schema);
        started = true;
    }

    // ---- viewport and events ----

    void sendViewport(int wPx, int hPx, int top, int bottom, int left, int right, boolean dark) {
        if (!started || wPx == 0) return;
        Render.Viewport v = new Render.Viewport();
        v.w = wPx / density;
        v.h = hPx / density;
        v.scale = density;
        v.safeTop = top / density;
        v.safeBottom = bottom / density;
        v.safeLeft = left / density;
        v.safeRight = right / density;
        v.platform = "android";
        v.dark = dark;
        if (lastViewport != null && lastViewport.w == v.w && lastViewport.h == v.h && lastViewport.safeTop == v.safeTop
                && lastViewport.safeBottom == v.safeBottom && lastViewport.dark == v.dark) {
            return;
        }
        lastViewport = v;
        Bridge.send(Render.encode(v));
    }

    /** Sends a press as if the user tapped the view (for tests and demos). */
    public void press(int tag) {
        Render.Press p = new Render.Press();
        p.tag = tag;
        Bridge.send(Render.encode(p));
    }

    /** Tells Go the user went back in a stack. */
    void popped(int tag, int depth) {
        Render.Popped p = new Render.Popped();
        p.tag = tag;
        p.depth = depth;
        Bridge.send(Render.encode(p));
    }

    /** Tells Go the size a stack gives its screens (px → dp). */
    void resized(int tag, int w, int h) {
        Render.Resized m = new Render.Resized();
        m.tag = tag;
        m.w = w / density;
        m.h = h / density;
        Bridge.send(Render.encode(m));
    }

    /**
     * The system back button: goes back in the deepest stack that can. Returns false when there is
     * nowhere to go back to, so the Activity can finish.
     */
    public boolean back() {
        NNStack best = null;
        for (View v : views.values()) {
            if (v instanceof NNStack && v.isShown() && ((NNStack) v).depth() > 1) best = (NNStack) v;
        }
        return best != null && best.goBack();
    }

    /** Reports a List's scroll position (px from the scroller) in dp. */
    void scrolled(int tag, int x, int y, int w, int h) {
        Render.Scrolled s = new Render.Scrolled();
        s.tag = tag;
        s.x = x / density;
        s.y = y / density;
        s.w = w / density;
        s.h = h / density;
        Bridge.send(Render.encode(s));
    }

    /** The tag of the view with this test ID, or 0. */
    public int tagForTestID(String id) {
        for (Map.Entry<Integer, View> e : views.entrySet()) {
            if (id.equals(e.getValue().getTag())) return e.getKey();
        }
        return 0;
    }

    public View viewForTag(int tag) {
        View v = views.get(tag);
        return v instanceof NNScroll ? ((NNScroll) v).scroller() : v;
    }

    int[] frameOf(View child) {
        int[] f = frames.get(child.getId());
        return f != null ? f : NO_FRAME;
    }

    View rootView() {
        return views.get(rootTag);
    }

    // ---- applying commits ----

    private void apply(Render.Commit c) {
        long t0 = System.nanoTime();
        Set<View> moved = new HashSet<>();
        for (Render.Op op : c.ops) {
            if (op instanceof Render.Create) {
                create((Render.Create) op);
            } else if (op instanceof Render.Destroy) {
                // Detach only the top of each destroyed subtree: its descendants go with it.
                // A screen sliding out of a stack goes when its animation ends.
                Set<View> gone = new HashSet<>();
                for (long tag : ((Render.Destroy) op).tags) {
                    View v = views.get((int) tag);
                    if (v != null) gone.add(v);
                }
                for (long tag : ((Render.Destroy) op).tags) {
                    View v = views.remove((int) tag);
                    frames.remove((int) tag);
                    if (v == null || !(v.getParent() instanceof ViewGroup)) continue;
                    ViewGroup parent = (ViewGroup) v.getParent();
                    if (gone.contains(parent) || gone.contains(contentOwner(parent))) continue;
                    if (parent instanceof NNStack && ((NNStack) parent).isLeaving(v)) continue;
                    parent.removeView(v);
                }
            } else if (op instanceof Render.Children) {
                children((Render.Children) op);
            } else if (op instanceof Render.Frame) {
                Render.Frame f = (Render.Frame) op;
                frames.put((int) f.tag, new int[] {Math.round(f.x * density), Math.round(f.y * density),
                        Math.round(f.w * density), Math.round(f.h * density)});
                View v = views.get((int) f.tag);
                if (v != null) moved.add(v);
            } else if (op instanceof Render.Box) {
                box((Render.Box) op);
            } else if (op instanceof Render.Text) {
                text((Render.Text) op);
            } else if (op instanceof Render.Image) {
                View v = views.get((int) ((Render.Image) op).tag);
                if (v instanceof NNImage) image((NNImage) v, (Render.Image) op);
            } else if (op instanceof Render.Input) {
                View v = views.get((int) ((Render.Input) op).tag);
                if (v instanceof NNInput) ((NNInput) v).apply((Render.Input) op, density);
            } else if (op instanceof Render.Scroll) {
                Render.Scroll s = (Render.Scroll) op;
                View v = views.get((int) s.tag);
                if (v instanceof NNScroll) ((NNScroll) v).setContent(Math.round(s.w * density), Math.round(s.h * density), s.horizontal, s.report);
            } else if (op instanceof Render.Screen) {
                Render.Screen sc = (Render.Screen) op;
                View v = views.get((int) sc.tag);
                if (v != null && v.getParent() instanceof NNStack) ((NNStack) v.getParent()).setTitle(v, sc.title);
                else if (v != null) pendingTitles.put(v, sc.title);
            } else if (op instanceof Render.Spinner) {
                Render.Spinner s = (Render.Spinner) op;
                View v = views.get((int) s.tag);
                if (v instanceof ProgressBar && s.color != 0) {
                    ((ProgressBar) v).setIndeterminateTintList(ColorStateList.valueOf(argb(s.color)));
                }
            }
        }
        if (c.root != 0) {
            rootTag = (int) c.root;
            root.setRoot(views.get(rootTag));
        }
        for (View v : moved) {
            v.requestLayout();
            if (v.getParent() instanceof View) ((View) v.getParent()).requestLayout();
        }
        if (onCommit != null) onCommit.onCommit(c, (System.nanoTime() - t0) / 1e6);
    }

    /** For a scroll view's content group (content → platform scroller → NNScroll), the NNScroll. */
    private static View contentOwner(View v) {
        if (v.getParent() instanceof android.widget.FrameLayout && v.getParent().getParent() instanceof NNScroll) {
            return (View) v.getParent().getParent();
        }
        return v;
    }

    private void create(Render.Create o) {
        int tag = (int) o.tag;
        View v;
        switch (o.kind) {
            case Render.Kind.TEXT: {
                TextView t = new TextView(ctx);
                t.setPadding(0, 0, 0, 0);
                t.setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE);
                t.setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE);
                t.setTextColor(0xff111111);
                v = t;
                break;
            }
            case Render.Kind.IMAGE:
                v = new NNImage(ctx);
                break;
            case Render.Kind.PRESSABLE:
                v = new NNPressable(ctx, this, tag);
                break;
            case Render.Kind.SCROLL:
                v = new NNScroll(ctx, this);
                break;
            case Render.Kind.INPUT:
                v = new NNInput(ctx, this, tag);
                break;
            case Render.Kind.STACK:
                v = new NNStack(ctx, this, tag);
                break;
            case Render.Kind.SPINNER:
                ProgressBar p = new ProgressBar(ctx);
                p.setIndeterminate(true);
                v = p;
                break;
            default:
                v = new NNGroup(ctx, this);
        }
        v.setId(tag);
        views.put(tag, v);
    }

    /** Makes a view's managed children exactly these, in order. */
    private void children(Render.Children o) {
        View pv = views.get((int) o.tag);
        if (pv == null) return;
        if (pv instanceof NNStack) {
            NNStack stack = (NNStack) pv;
            java.util.List<View> screens = new java.util.ArrayList<>();
            for (long t : o.children) {
                View v = views.get((int) t);
                if (v != null) screens.add(v);
            }
            stack.setScreens(screens);
            for (View v : screens) {
                String title = pendingTitles.remove(v);
                if (title != null) stack.setTitle(v, title);
            }
            return;
        }
        ViewGroup parent = pv instanceof NNScroll ? ((NNScroll) pv).content : (ViewGroup) pv;
        Set<View> keep = new HashSet<>();
        View[] wanted = new View[o.children.size()];
        int n = 0;
        for (long t : o.children) {
            View v = views.get((int) t);
            if (v != null) {
                wanted[n++] = v;
                keep.add(v);
            }
        }
        for (int i = parent.getChildCount() - 1; i >= 0; i--) {
            if (!keep.contains(parent.getChildAt(i))) parent.removeViewAt(i);
        }
        for (int i = 0; i < n; i++) {
            View v = wanted[i];
            if (i < parent.getChildCount() && parent.getChildAt(i) == v) continue;
            if (v.getParent() != null) ((ViewGroup) v.getParent()).removeView(v);
            parent.addView(v, i);
        }
    }

    private void box(Render.Box o) {
        View v = views.get((int) o.tag);
        if (v == null) return;
        if (o.background != 0 || o.radius != 0 || o.borderWidth != 0) {
            GradientDrawable g = new GradientDrawable();
            g.setColor(argb(o.background));
            g.setCornerRadius(o.radius * density);
            if (o.borderWidth > 0) g.setStroke(Math.max(1, Math.round(o.borderWidth * density)), argb(o.borderColor));
            v.setBackground(g);
        } else if (!(v instanceof ProgressBar)) {
            v.setBackground(null);
        }
        if (v instanceof NNPressable) {
            ((NNPressable) v).baseAlpha = o.opacity;
        }
        v.setAlpha(o.opacity);
        v.setClipToOutline(o.clip);
        if (v instanceof NNGroup && !(v instanceof NNScroll)) ((NNGroup) v).setClipChildren(o.clip); // scroll views always clip
        v.setContentDescription(o.label.isEmpty() ? null : o.label);
        v.setTag(o.testID.isEmpty() ? null : o.testID);
    }

    private void text(Render.Text o) {
        View v = views.get((int) o.tag);
        if (!(v instanceof TextView)) return;
        TextView t = (TextView) v;
        t.setText(o.text);
        t.setTextSize(TypedValue.COMPLEX_UNIT_PX, o.font.size * density);
        t.setTypeface(typeface(o.font.weight));
        if (o.color != 0) t.setTextColor(argb(o.color));
        int g = o.align == Render.Align.CENTER ? Gravity.CENTER_HORIZONTAL : o.align == Render.Align.END ? Gravity.END : Gravity.START;
        t.setGravity(g | Gravity.TOP);
        if (o.lines > 0) {
            t.setMaxLines((int) o.lines);
            t.setEllipsize(TextUtils.TruncateAt.END);
        } else {
            t.setMaxLines(Integer.MAX_VALUE);
            t.setEllipsize(null);
        }
    }

    // ---- images ----

    private static final ExecutorService loader = Executors.newFixedThreadPool(4);
    private static final LruCache<String, Bitmap> bitmaps = new LruCache<String, Bitmap>(16 << 20) {
        @Override
        protected int sizeOf(String key, Bitmap b) {
            return b.getByteCount();
        }
    };
    private static final Handler main = new Handler(Looper.getMainLooper());

    /** An image view that remembers which URL it shows. */
    static final class NNImage extends ImageView {
        String url;

        NNImage(Context c) {
            super(c);
        }
    }

    private void image(NNImage v, Render.Image o) {
        v.setScaleType(o.fit == Render.Fit.CONTAIN ? ImageView.ScaleType.FIT_CENTER
                : o.fit == Render.Fit.STRETCH ? ImageView.ScaleType.FIT_XY : ImageView.ScaleType.CENTER_CROP);
        String url = o.url;
        if (url.equals(v.url)) return;
        v.url = url;
        v.setImageDrawable(null);
        Bitmap hit = bitmaps.get(url);
        if (hit != null) {
            v.setImageBitmap(hit);
            return;
        }
        loader.execute(() -> {
            try {
                HttpURLConnection c = (HttpURLConnection) new URL(url).openConnection();
                c.setInstanceFollowRedirects(true);
                try (InputStream in = c.getInputStream()) {
                    Bitmap b = BitmapFactory.decodeStream(in);
                    if (b == null) return;
                    bitmaps.put(url, b);
                    main.post(() -> {
                        if (url.equals(v.url)) v.setImageBitmap(b);
                    });
                }
            } catch (Exception e) {
                android.util.Log.w("NimbusNative", "image " + url + ": " + e);
            }
        });
    }

    // ---- answering Go (on a Go thread) ----

    /** Measures text the way TextView draws it. */
    static byte[] answer(byte[] req, float d) {
        Render.Message m = Render.decode(req);
        if (!(m instanceof Render.MeasureText)) return null;
        Render.MeasureText mt = (Render.MeasureText) m;
        TextPaint p = new TextPaint(Paint.ANTI_ALIAS_FLAG);
        p.setTextSize(mt.font.size * d);
        p.setTypeface(typeface(mt.font.weight));
        String text = mt.text;
        int desired = (int) Math.ceil(Layout.getDesiredWidth(text, p));
        int width = mt.maxWidth < 0 ? desired : Math.min(desired, (int) Math.floor(mt.maxWidth * d));
        StaticLayout.Builder b = StaticLayout.Builder.obtain(text, 0, text.length(), p, Math.max(width, 1))
                .setIncludePad(true)
                .setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE)
                .setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE);
        if (mt.lines > 0) b.setMaxLines((int) mt.lines).setEllipsize(TextUtils.TruncateAt.END);
        StaticLayout l = b.build();
        float w = 0;
        for (int i = 0; i < l.getLineCount(); i++) w = Math.max(w, l.getLineWidth(i));
        Render.TextSize ts = new Render.TextSize();
        ts.w = (float) Math.ceil(w) / d;
        ts.h = l.getHeight() / d;
        return Render.encode(ts);
    }

    static Typeface typeface(int weight) {
        int w = weight == 0 ? 400 : weight;
        if (Build.VERSION.SDK_INT >= 28) return Typeface.create(Typeface.DEFAULT, w, false);
        return w >= 600 ? Typeface.DEFAULT_BOLD : Typeface.DEFAULT;
    }

    /** 0xRRGGBBAA → Android's 0xAARRGGBB. */
    static int argb(long c) {
        int v = (int) c;
        return (v >>> 8) | (v << 24);
    }

    /** A single-line text field that reports edits (but not the runtime's own updates). */
    static final class NNInput extends EditText {
        private boolean applying;

        NNInput(Context c, Renderer r, int tag) {
            super(c);
            setSingleLine(true);
            setBackground(null);
            int pad = Math.round(10 * r.density); // the runtime measures inputs with 10 dp padding
            setPadding(pad, pad, pad, pad);
            addTextChangedListener(new TextWatcher() {
                @Override
                public void beforeTextChanged(CharSequence s, int a, int b, int n) {}

                @Override
                public void onTextChanged(CharSequence s, int a, int b, int n) {}

                @Override
                public void afterTextChanged(Editable s) {
                    if (applying) return;
                    Render.Changed ch = new Render.Changed();
                    ch.tag = tag;
                    ch.text = s.toString();
                    Bridge.send(Render.encode(ch));
                }
            });
        }

        void apply(Render.Input o, float d) {
            applying = true;
            if (!getText().toString().equals(o.value)) {
                setText(o.value);
                setSelection(o.value.length());
            }
            applying = false;
            setHint(o.placeholder.isEmpty() ? null : o.placeholder);
            setTextSize(TypedValue.COMPLEX_UNIT_PX, o.font.size * d);
            setTypeface(typeface(o.font.weight));
            if (o.color != 0) setTextColor(argb(o.color));
            int type = InputType.TYPE_CLASS_TEXT;
            if (o.keyboard == Render.Keyboard.NUMBER) type = InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_FLAG_DECIMAL;
            else if (o.keyboard == Render.Keyboard.EMAIL) type |= InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS;
            else if (o.keyboard == Render.Keyboard.URL) type |= InputType.TYPE_TEXT_VARIATION_URI;
            if (o.secure) type = InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD;
            if (getInputType() != type) setInputType(type);
        }
    }
}
