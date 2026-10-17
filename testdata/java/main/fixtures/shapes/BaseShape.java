package fixtures.shapes;

public abstract class BaseShape implements Shape {
    protected final String name;

    protected BaseShape(String name) { this.name = name; }

    @Override public abstract double area();

    @Override public String describe() { return name; }

    public double scaled(double k) { return area() * k; }    // invoke-virtual on abstract method
}
