# Autopus-ADK Hero Demo
# Usage: cd demo && vhs hero.tape
# Output: hero.gif

Output hero.gif

Require auto

Set Shell "zsh"
Set FontSize 16
Set Width 1400
Set Height 750
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 40ms
Set WindowBar Colorful
Set WindowBarSize 40
Set CursorBlink true

# Step 1: Version
Type "auto version"
Enter
Sleep 2s

# Step 2: Doctor — health check
Type "auto doctor"
Enter
Sleep 3s

# Step 3: Platform detection
Type "auto platform list"
Enter
Sleep 2.5s

# Step 4: SPEC Dashboard
Type "auto status"
Enter
Sleep 4s

# Step 5: Skill list
Type "auto skill list | head -20"
Enter
Sleep 3s

# Final message
Type ""
Enter
Type "echo '🐙 One harness. 15 agents. 35 skills. Every platform.'"
Enter
Sleep 3s
