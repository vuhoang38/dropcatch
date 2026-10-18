import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHeader } from '@/components/shared/page-header'
import { Button } from '@/components/ui/button'

describe('PageHeader', () => {
  it('renders the title as the page heading', () => {
    render(<PageHeader title="Users" />)

    expect(screen.getByRole('heading', { level: 1, name: 'Users' })).toBeInTheDocument()
  })

  it('omits the description when none is given', () => {
    render(<PageHeader title="Users" />)

    expect(screen.queryByText(/everyone with access/i)).not.toBeInTheDocument()
  })

  it('renders actions beside the title', () => {
    render(<PageHeader title="Users" actions={<Button>Invite</Button>} />)

    expect(screen.getByRole('button', { name: 'Invite' })).toBeInTheDocument()
  })
})
