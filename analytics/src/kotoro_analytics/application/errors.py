class NotFoundError(Exception):
    """The caller has nothing to see for this resource.

    Used for both "does not exist" and "belongs to another user", so responses never
    reveal that another user's resource exists (ADR 0003).
    """
