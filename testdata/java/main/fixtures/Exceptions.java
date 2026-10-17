package fixtures;

import java.io.IOException;

/**
 * try/catch with multiple typed handlers, catch-all, finally, nested try, re-throw,
 * explicit throw, synchronized blocks (monitor-enter/monitor-exit) and methods.
 */
public class Exceptions {
    private final Object lock = new Object();
    private int counter;

    public static int multiCatch(String s) {
        try {
            return Integer.parseInt(s);
        } catch (NumberFormatException e) {
            return -1;
        } catch (NullPointerException e) {
            return -2;
        } catch (RuntimeException e) {
            return -3;
        }
    }

    public static int multiCatchUnion(String s) {
        try {
            return Integer.parseInt(s) / s.length();
        } catch (NumberFormatException | ArithmeticException e) {
            return -1;
        }
    }

    public static int catchAll(Runnable r) {
        try {
            r.run();
            return 0;
        } catch (Throwable t) {
            return 1;
        }
    }

    public static int finallyBlock(int[] arr, int i) {
        int r = 0;
        try {
            r = arr[i];
        } finally {
            r += 100;
        }
        return r;
    }

    public static int tryCatchFinally(String s) {
        int r = 0;
        try {
            r = Integer.parseInt(s);
        } catch (NumberFormatException e) {
            r = -1;
        } finally {
            r += 1000;
        }
        return r;
    }

    public static int nested(String a, String b) {
        try {
            int x = Integer.parseInt(a);
            try {
                return x / Integer.parseInt(b);
            } catch (ArithmeticException e) {
                return -1;
            }
        } catch (NumberFormatException e) {
            return -2;
        }
    }

    public static int nestedFinally(int[] a, int i) {
        int r = 0;
        try {
            try {
                r = a[i];
            } finally {
                r += 1;
            }
        } catch (ArrayIndexOutOfBoundsException e) {
            r = -1;
        } finally {
            r += 10;
        }
        return r;
    }

    public static int rethrow(String s) {
        try {
            return Integer.parseInt(s);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("bad: ".concat(s), e);
        }
    }

    public static int rethrowSame(String s) throws IOException {
        try {
            if (s == null) throw new IOException("null");
            return s.length();
        } catch (IOException e) {
            throw e;                                   // move-exception + throw
        }
    }

    public static void throwsExplicit(boolean b) {
        if (b) throw new IllegalStateException("explicit");
    }

    public static void throwsChecked() throws IOException {
        throw new IOException("io");
    }

    public static int catchInLoop(String[] xs) {
        int ok = 0;
        for (String x : xs) {
            try {
                ok += Integer.parseInt(x);
            } catch (NumberFormatException e) {
                continue;
            }
        }
        return ok;
    }

    public static Object catchReturnsException(String s) {
        try {
            return Class.forName(s);
        } catch (ClassNotFoundException e) {
            return e;
        }
    }

    public int synchronizedBlock() {
        synchronized (lock) {                          // monitor-enter / monitor-exit
            return ++counter;
        }
    }

    public int synchronizedBlockWithThrow(int x) {
        synchronized (this) {
            if (x < 0) throw new IllegalArgumentException("neg");
            counter += x;
        }
        return counter;
    }

    public static int synchronizedStaticBlock(Object o) {
        synchronized (Exceptions.class) {
            return o.hashCode();
        }
    }

    public synchronized int synchronizedMethod() {     // ACC_SYNCHRONIZED
        return ++counter;
    }

    public static synchronized int staticSynchronized(int x) {
        return x + 1;
    }

    public static int finallyWithReturnInTry(int x) {
        try {
            if (x > 0) return 1;
            return 2;
        } finally {
            x = 0;
        }
    }

    public static String stackTrace() {
        try {
            throw new RuntimeException("trace");
        } catch (RuntimeException e) {
            StackTraceElement[] st = e.getStackTrace();
            return st.length > 0 ? st[0].getMethodName() : "";
        }
    }
}
