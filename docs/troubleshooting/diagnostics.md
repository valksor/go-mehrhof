# Diagnostics

Debugging tools and getting help.

## Check Version

```bash
mehr version
```

## Enable Verbose Mode

```bash
mehr --verbose <command>
```

## View Logs

```bash
cat .mehrhof/work/*/sessions/*.yaml
```

## Validate Configuration

```bash
mehr config validate
```

## Check Configuration File

```bash
cat .mehrhof/config.yaml
```

## Configuration Issues

### "Settings not applied"

**Cause:** Configuration not loaded or CLI flags overriding.

**Solution:**

```bash
# Validate config
mehr config validate

# Check config file
cat .mehrhof/config.yaml

# CLI flags override config settings
mehr --verbose plan  # verbose always enabled
```

### "Invalid configuration"

**Cause:** Malformed YAML or invalid values.

**Solution:**

```bash
# Validate YAML syntax
mehr config validate

# Or manually check
cat .mehrhof/config.yaml | python -c "import yaml,sys; yaml.safe_load(sys.stdin)"
```

## Report Issues

1. Note the error message
2. Check Mehrhof version
3. Gather relevant logs
4. Report at project issue tracker
