package fixtures.shapes;

public class Square extends BaseShape {
    protected final double side;
    public Square(double side) { super("square"); this.side = side; }
    @Override public double area() { return side * side; }
    @Override public String describe() { return "square:".concat(name); }
    /** Inherited by Rect without override. */
    public int corners() { return 4; }
}
