# Pulled transcript recovery

Pulls stage a unique complete file set before publishing it with filesystem
renames. A per-transcript operating-system lock serializes concurrent Peasant
processes. The previous directory remains in an owned backup until the new
files are validated and their SQLite index transaction succeeds.

Directory replacement uses a backup rename followed by a staging rename, not
a portable atomic exchange of two nonempty directories. An interruption between
those renames can leave the canonical directory absent until recovery; the
previous complete files remain in the backup throughout that interval.

If a write, publication, or index operation fails, the command returns an error.
It does not delete valid published content or claim a database rollback removed
files. Retry the normal pull to validate and finish an interrupted installation;
`peasant village transcripts pull <id> --force` also repairs the index and
downloads a fresh replacement. No manual SQL is required. An invalid journal or
checksum mismatch fails closed, retaining files rather than guessing which
unverified copy to delete.

An unchanged content hash or HTTP 304 does not freeze descriptive metadata.
Changed metadata is atomically replaced as a file, without moving the blob or
provenance, then indexed through the same recovery boundary. A metadata-only
failure retains the prior complete copy. Dry runs neither repair installations
nor write files or index rows.

Recovery cleans only its reserved per-transcript staging namespace and known
journal files, never legacy temporary directories or unrelated files. Pulled
manifests remain content provenance, not account-private collective observations;
downloaded foreign transcripts remain non-republishable.

File data and directory publication barriers are flushed on supported local
filesystems. Windows flushes file data but has no portable directory-flush API
through Go's `os.File`; process-interruption recovery still applies. Filesystem
or hardware durability guarantees remain platform-dependent.
