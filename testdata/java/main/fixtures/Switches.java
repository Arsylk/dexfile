package fixtures;

/** packed-switch, sparse-switch and String-switch (hashCode sparse-switch + equals). */
public class Switches {
    public static String packed(int x) {
        switch (x) {
            case 0: return "zero";
            case 1: return "one";
            case 2: return "two";
            case 3: return "three";
            case 4: return "four";
            case 5: return "five";
            case 6: return "six";
            case 7: return "seven";
            case 8: return "eight";
            case 9: return "nine";
            default: return "many";
        }
    }

    public static int packedFallthrough(int x) {
        int r = 0;
        switch (x) {
            case 10: r += 1;
            case 11: r += 2;
            case 12: r += 4; break;
            case 13: r += 8;
            case 14: r += 16; break;
            case 15: r += 32;
            case 16: r += 64; break;
            default: r = -1;
        }
        return r;
    }

    public static int sparse(int x) {
        switch (x) {
            case -5: return 1;
            case 1: return 2;
            case 100: return 3;
            case 10000: return 4;
            case 1000000: return 5;
            case Integer.MAX_VALUE: return 6;
            default: return 0;
        }
    }

    public static int sparseNoDefault(int x) {
        int r = 0;
        switch (x) {
            case 7: r = 70; break;
            case 77: r = 770; break;
            case 777: r = 7770; break;
            case 7777: r = 77770; break;
        }
        return r;
    }

    public static int stringSwitch(String s) {
        switch (s) {
            case "alpha": return 1;
            case "beta": return 2;
            case "gamma": return 3;
            case "delta": return 4;
            case "Aa": return 5;   // same hashCode as "BB"
            case "BB": return 6;
            default: return -1;
        }
    }

    public static int charSwitch(char c) {
        switch (c) {
            case 'a': return 1;
            case 'b': return 2;
            case 'c': return 3;
            case 'd': return 4;
            case 'e': return 5;
            default: return 0;
        }
    }

    public static int nestedSwitch(int a, int b) {
        switch (a) {
            case 0:
                switch (b) {
                    case 0: return 0;
                    case 1: return 1;
                    case 2: return 2;
                    case 3: return 3;
                    default: return -1;
                }
            case 1: return 10;
            case 2: return 20;
            case 3: return 30;
            default: return -10;
        }
    }

    public static int switchInLoop(int[] xs) {
        int acc = 0;
        for (int x : xs) {
            switch (x) {
                case 0: acc += 1; break;
                case 1: acc += 2; break;
                case 2: acc += 3; break;
                case 3: acc += 4; break;
                case 4: acc += 5; break;
                default: acc -= 1;
            }
        }
        return acc;
    }
}
