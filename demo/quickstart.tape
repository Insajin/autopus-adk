# Autopus-ADK Quick Start Demo
# Usage: cd demo && vhs quickstart.tape

Output quickstart.gif

Require auto

Set Shell "zsh"
Set FontSize 16
Set Width 1400
Set Height 700
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 45ms
Set WindowBar Colorful
Set WindowBarSize 40
Set CursorBlink true

# Step 1: Show version
Type "auto version"
Enter
Sleep 2s

# Step 2: Detect platforms
Type "auto platform list"
Enter
Sleep 2.5s

# Step 3: Health check
Type "auto doctor"
Enter
Sleep 4s

# Step 4: List skills
Type "auto skill list | head -15"
Enter
Sleep 3s

# Step 5: Show SPEC dashboard
Type "auto status | head -20"
Enter
Sleep 4s
