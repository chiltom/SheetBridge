export default {
  content: [
    "./web/templates/**/*.tmpl", // Path to Go HTML templates
    "./web/static/js/**/*.js", // Path to any JS files that may use Tailwind classes
  ],
  theme: {
    extend: {
      // Extend Tailwind's default theme here
    },
  },
  plugins: [
    require("daisyui"), // This line requires DaisyUI
    // For this to work, DaisyUI needs to be resolvable by Node's 'require'.
  ],
  // Optional: DaisyUI specific configurations
  daisyui: {
    themes: ["cupcake", "dark", "light"], // Add or remove themes
    styled: true, // Include DaisyUI's opinionated styles
    base: true, // Include Tailwind base styles (preflight)
    utils: true, // Include DaisyUI utility classes
    logs: true, // Show DaisyUI logs during build (useful for debugging)
    rtl: false, // Right-to-left support
    prefix: "", // Prefix for DaisyUI classes (e.g., "dui-")
    darkTheme: "dark", // Default dark theme
  },
};
