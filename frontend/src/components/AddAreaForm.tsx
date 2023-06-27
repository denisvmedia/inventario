import React, { useState } from 'react';
import api from '../api/api';

function AddAreaForm() {
  const [name, setName] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!name) {
      setError('Please enter the area name.');
      return;
    }

    setIsLoading(true);

    try {
      const response = await api.post('/areas', {
        data: {
          name,
        },
      });

      // eslint-disable-next-line no-console
      console.log('New area created:', response.data);
      setName('');
    } catch (error: any) {
      // eslint-disable-next-line no-console
      console.error('Error creating area:', error);
      if (error.response && error.response.data && error.response.data.errors) {
        const errorMessage = error.response.data.errors[0].error.error.data.location_id;
        setError(`location_id: ${errorMessage}`);
      } else {
        setError('Error creating area. Please try again.');
      }
    }

    setIsLoading(false);
  };

  return (
    <div>
      <h2>Add New Area</h2>
      {error && <div>{error}</div>}
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="Area Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <button type="submit" disabled={isLoading}>
          {isLoading ? 'Adding...' : 'Add Area'}
        </button>
      </form>
    </div>
  );
}

export default AddAreaForm;
