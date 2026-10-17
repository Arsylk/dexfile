package fixtures.shapes;

public class ShapeUser {
    public static double viaInterface(Shape s) { return s.area(); }              // invoke-interface Shape.area
    public static String describeViaInterface(Shape s) { return s.describe(); }
    public static double viaAbstract(BaseShape b) { return b.area(); }           // invoke-virtual BaseShape.area
    public static double viaSquare(Square s) { return s.area(); }                // Square or Rect
    public static double viaRect(Rect r) { return r.area(); }
    public static String inheritedDescribe(Rect r) { return r.describe(); }      // resolves to Square.describe
    public static int inheritedCorners(Rect r) { return r.corners(); }           // resolves to Square.corners
    public static double scaled(BaseShape b) { return b.scaled(2.0); }

    public static double all() {
        Shape[] shapes = {new Circle(1), new Square(2), new Rect(2, 3)};
        double t = 0;
        for (Shape s : shapes) t += s.area();
        return t;
    }

    public static Shape pick(int k) {
        if (k == 0) return new Circle(1);
        if (k == 1) return new Square(1);
        return new Rect(1, 2);
    }
}
