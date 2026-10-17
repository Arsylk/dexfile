package fixtures.shapes;

public class Circle extends BaseShape {
    private final double r;
    public Circle(double r) { super("circle"); this.r = r; }
    @Override public double area() { return Math.PI * r * r; }
}
