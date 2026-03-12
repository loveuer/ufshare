import { useEffect, useState } from 'react'
import {
  Box, Card, CardContent, Typography, Alert,
  CircularProgress, Paper, Table,
  TableBody, TableCell, TableContainer, TableHead, TableRow,
  TablePagination, Button, TextField, Chip,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { mavenApi } from '../api'
import type { MavenArtifact } from '../types'

export default function MavenPage() {
  const [artifacts, setArtifacts] = useState<MavenArtifact[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(20)
  const [search, setSearch] = useState('')
  const [groupIdFilter, setGroupIdFilter] = useState('')
  const [artifactIdFilter, setArtifactIdFilter] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const loadArtifacts = async () => {
    setLoading(true)
    setError('')
    try {
      let res
      if (search) {
        res = await mavenApi.searchArtifacts(search, page + 1, rowsPerPage)
      } else {
        res = await mavenApi.listArtifacts(page + 1, rowsPerPage, groupIdFilter, artifactIdFilter)
      }
      setArtifacts(res.data.data || [])
      setTotal((res.data as unknown as { total: number }).total || 0)
    } catch {
      setError('Failed to load artifacts')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadArtifacts()
  }, [page, rowsPerPage])

  const handleSearch = () => {
    setPage(0)
    loadArtifacts()
  }

  const formatCoordinates = (artifact: MavenArtifact) => {
    return `${artifact.group_id}:${artifact.artifact_id}:${artifact.version}`
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Maven Repository
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Card sx={{ mb: 2 }}>
        <CardContent>
          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
            <TextField
              size="small"
              label="Search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
              placeholder="Search artifacts..."
              sx={{ minWidth: 200 }}
            />
            <TextField
              size="small"
              label="Group ID"
              value={groupIdFilter}
              onChange={(e) => setGroupIdFilter(e.target.value)}
              placeholder="com.example"
              sx={{ minWidth: 150 }}
            />
            <TextField
              size="small"
              label="Artifact ID"
              value={artifactIdFilter}
              onChange={(e) => setArtifactIdFilter(e.target.value)}
              placeholder="myapp"
              sx={{ minWidth: 150 }}
            />
            <Button
              variant="contained"
              startIcon={<SearchIcon />}
              onClick={handleSearch}
            >
              Search
            </Button>
          </Box>
        </CardContent>
      </Card>

      <Paper>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Coordinates (GAV)</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Source</TableCell>
                <TableCell>Uploader</TableCell>
                <TableCell>Created</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} align="center">
                    <CircularProgress size={24} />
                  </TableCell>
                </TableRow>
              ) : artifacts.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center">
                    No artifacts found
                  </TableCell>
                </TableRow>
              ) : (
                artifacts.map((artifact) => (
                  <TableRow key={artifact.id}>
                    <TableCell>
                      <Typography variant="body2" fontFamily="monospace">
                        {formatCoordinates(artifact)}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      {artifact.is_snapshot ? (
                        <Chip size="small" color="warning" label="SNAPSHOT" />
                      ) : (
                        <Chip size="small" color="success" label="Release" />
                      )}
                    </TableCell>
                    <TableCell>
                      {artifact.is_uploaded ? (
                        <Chip size="small" color="primary" label="Uploaded" />
                      ) : (
                        <Chip size="small" label="Proxied" />
                      )}
                    </TableCell>
                    <TableCell>{artifact.uploader || '-'}</TableCell>
                    <TableCell>
                      {new Date(artifact.created_at).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(parseInt(e.target.value, 10))
            setPage(0)
          }}
        />
      </Paper>
    </Box>
  )
}
