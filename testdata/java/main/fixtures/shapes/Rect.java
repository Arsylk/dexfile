package fixtures.shapes;

public class Rect extends Square {
    private final double width;
    public Rect(double side, double width) { super(side); this.width = width; }
    @Override public double area() { return side * width; }
    // describe() and corners() are inherited from Square, not overridden.
}
