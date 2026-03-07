# Theming Verification Checklist

## Pre-Restart Check

✓ Custom gruvbox theme defined in orchestrator.kdl (lines 9-142)
✓ Theme includes frame_unselected component (lines 87-94)
✓ Theme includes frame_selected with orchestrator color #d79921 (lines 95-102)
✓ Theme selection set to "gruvbox" (line 142)
✓ Documentation updated in docs/theming.md
✓ Standalone theme file created in themes/gruvbox.kdl

## After Restart Verification

To verify the theming changes work correctly:

1. **Restart the orchestrator session**
   ```bash
   ./restart-zellij.sh
   ```

2. **Check orchestrator tab frames**
   - [ ] YakMap pane (left) has a visible frame
   - [ ] Claude (Yakob) pane (top right) has a frame
   - [ ] Shell pane (bottom right) has a frame
   - [ ] Focused pane shows yellow/gold frame (#d79921 color)
   - [ ] Unfocused panes show subtle gray frames

3. **Test focus switching**
   - [ ] Press Ctrl+P to enter PANE mode
   - [ ] Use arrow keys to switch between panes
   - [ ] Verify frame color changes: focused = yellow, unfocused = gray
   - [ ] Press ESC to return to normal mode

4. **Check worker tabs** (if workers exist)
   - [ ] Worker shell panes have visible frames
   - [ ] Frame styling matches orchestrator tab
   - [ ] Focus indication works consistently

5. **Visual consistency check**
   - [ ] All pane frames appear similar in style
   - [ ] No panes are missing frames
   - [ ] Color scheme is consistent across all panes
   - [ ] Selected pane frame color matches Claude accent color

## Expected Visual Behavior

### Focused Pane
- Frame color: Warm yellow/gold (RGB 215 153 33 / #d79921)
- Matches the orchestrator agent accent color
- Stands out clearly from other panes

### Unfocused Panes
- Frame color: Subtle gray (RGB 102 92 84 / #665c54)
- Provides visual separation without being distracting
- Clear hierarchy: focused stands out, unfocused recede

### Consistency
- Shell panes, YakMap, and TUI panes all have the same frame styling
- No visual distinction between "terminal" and "TUI" panes
- Unified appearance across the orchestrator interface

## Troubleshooting

If frames don't appear correctly:

1. **Check Zellij version**
   ```bash
   zellij --version  # Should be 0.43.1 or newer
   ```

2. **Verify theme loaded**
   - Press Ctrl+O for session menu
   - Press 'c' for configuration
   - Look for active theme name (should show "gruvbox")

3. **Check for theme errors**
   ```bash
   grep -i error ~/.cache/zellij/zellij.log
   ```

4. **Validate KDL syntax**
   ```bash
   head -150 orchestrator.kdl | tail -10  # Should show theme definition closing
   ```

5. **Fallback: Use standalone theme**
   If inline theme fails, Zellij should fall back to `themes/gruvbox.kdl`

## Success Criteria

✅ All panes have consistent frame styling
✅ Focused pane uses orchestrator accent color
✅ Unfocused panes have subtle but visible frames
✅ Frame colors match Claude accent color aesthetic
✅ Visual hierarchy is clear and consistent

